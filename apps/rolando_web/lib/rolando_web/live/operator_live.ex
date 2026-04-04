defmodule RolandoWeb.OperatorLive do
  use RolandoWeb, :live_view

  alias Rolando.Analytics
  alias Rolando.Analytics.Sync
  alias Rolando.Broadcast
  alias Rolando.Contexts.Guilds

  @guild_page_size 15
  @broadcast_cooldown_sec 3
  @chart_days 7

  @impl true
  def mount(_params, _session, socket) do
    if connected?(socket) do
      Phoenix.PubSub.subscribe(Rolando.PubSub, Sync.ui_topic())
      :timer.send_interval(5000, :memory_tick)
    end

    socket =
      socket
      |> assign(
        :event_filter_form,
        to_form(%{"event_type" => "", "guild_id" => "", "since" => ""}, as: :event_filters)
      )
      |> load_analytics()
      |> load_guild_page(1)
      |> assign_chart()
      |> assign(:memory_mb, beam_memory_mb())
      |> assign(:memory_pct, memory_bar_pct())
      |> assign(
        :broadcast_form,
        to_form(
          %{"body" => "", "channel_ids_raw" => "", "user_ids_raw" => "", "guild_id" => ""},
          as: :broadcast
        )
      )
      |> assign(:last_broadcast_at, nil)
      |> assign(:chart_days, @chart_days)

    {:ok, socket}
  end

  defp load_analytics(socket) do
    filters = parse_event_filters(socket.assigns.event_filter_form.params)
    assign(socket, :events, Analytics.list_recent_events(100, filters))
  end

  defp parse_event_filters(%{"event_type" => et, "guild_id" => gid, "since" => sd}) do
    %{}
    |> maybe_put_trim(:event_type, et)
    |> maybe_put_trim(:guild_id, gid)
    |> maybe_put_since(sd)
  end

  defp parse_event_filters(_), do: %{}

  defp maybe_put_trim(acc, k, v) do
    v = v |> to_string() |> String.trim()
    if v == "", do: acc, else: Map.put(acc, k, v)
  end

  defp maybe_put_since(acc, sd) do
    sd = sd |> to_string() |> String.trim()

    if sd == "" do
      acc
    else
      case Date.from_iso8601(sd) do
        {:ok, d} -> Map.put(acc, :since, DateTime.new!(d, ~T[00:00:00], "Etc/UTC"))
        _ -> acc
      end
    end
  end

  defp load_guild_page(socket, page) do
    page = max(1, page)
    count = Guilds.count_guilds()
    total_pages = max(1, div(count + @guild_page_size - 1, @guild_page_size))
    page = min(page, total_pages)

    rows = Guilds.list_directory_page(page, @guild_page_size)

    socket
    |> assign(:guild_page, page)
    |> assign(:guild_total, count)
    |> assign(:guild_total_pages, total_pages)
    |> assign(:guild_rows, rows)
  end

  defp assign_chart(socket) do
    series = chart_series()
    max_c = series |> Enum.map(&elem(&1, 1)) |> Enum.max(fn -> 1 end)

    socket
    |> assign(:chart_series, series)
    |> assign(:chart_max, max_c)
  end

  defp chart_series do
    raw = Analytics.event_counts_by_day(@chart_days)
    fill_days(raw, @chart_days)
  end

  defp fill_days(series, days) do
    map = Map.new(series, fn {d, n} -> {d, n} end)
    today = Date.utc_today()

    for i <- (days - 1)..0//-1 do
      d = today |> Date.add(-i) |> Date.to_string()
      {d, Map.get(map, d, 0)}
    end
  end

  defp beam_memory_mb do
    bytes = :erlang.memory(:total)
    Float.round(bytes / 1_048_576, 1)
  end

  defp memory_bar_pct do
    bytes = :erlang.memory(:total)
    cap = 512 * 1024 * 1024
    min(100.0, bytes / cap * 100.0)
  end

  @impl true
  def handle_info(:analytics_updated, socket) do
    page = socket.assigns.guild_page

    {:noreply,
     socket
     |> load_analytics()
     |> assign_chart()
     |> assign(:guild_rows, Guilds.list_directory_page(page, @guild_page_size))}
  end

  def handle_info(:memory_tick, socket) do
    {:noreply,
     socket
     |> assign(:memory_mb, beam_memory_mb())
     |> assign(:memory_pct, memory_bar_pct())}
  end

  def handle_info(_, socket), do: {:noreply, socket}

  @impl true
  def handle_event("filter_events", %{"event_filters" => params}, socket) do
    {:noreply,
     socket
     |> assign(:event_filter_form, to_form(params, as: :event_filters))
     |> load_analytics()}
  end

  def handle_event("guild_page", %{"dir" => dir}, socket) do
    page = socket.assigns.guild_page

    next =
      case dir do
        "prev" -> max(1, page - 1)
        "next" -> min(socket.assigns.guild_total_pages, page + 1)
        _ -> page
      end

    {:noreply, load_guild_page(socket, next)}
  end

  def handle_event("broadcast", %{"broadcast" => params}, socket) do
    now = System.monotonic_time(:second)
    last = socket.assigns.last_broadcast_at

    if last && now - last < @broadcast_cooldown_sec do
      {:noreply, put_flash(socket, :error, "Wait a few seconds between broadcasts.")}
    else
      case validate_broadcast(params) do
        {:ok, body, channel_ids, user_ids, guild_id} ->
          correlation_id = Ecto.UUID.generate()

          envelope = %{
            correlation_id: correlation_id,
            body: body,
            channel_ids: channel_ids,
            user_ids: user_ids,
            guild_id: guild_id,
            operator_user_id: socket.assigns.current_scope.user_id
          }

          Broadcast.publish(envelope)

          {:noreply,
           socket
           |> put_flash(:info, "Broadcast queued for delivery.")
           |> assign(:last_broadcast_at, now)
           |> assign(
             :broadcast_form,
             to_form(
               %{"body" => "", "channel_ids_raw" => "", "user_ids_raw" => "", "guild_id" => ""},
               as: :broadcast
             )
           )}

        {:error, msg} ->
          {:noreply, put_flash(socket, :error, msg)}
      end
    end
  end

  defp validate_broadcast(params) do
    body = String.trim(to_string(params["body"] || ""))

    guild_id =
      case String.trim(to_string(params["guild_id"] || "")) do
        "" -> nil
        g -> g
      end

    channel_ids =
      params["channel_ids_raw"]
      |> to_string()
      |> split_ids()

    user_ids =
      params["user_ids_raw"]
      |> to_string()
      |> split_ids()

    cond do
      body == "" ->
        {:error, "Message body is required."}

      String.length(body) > 2000 ->
        {:error, "Message body must be at most 2000 characters."}

      channel_ids == [] and user_ids == [] ->
        {:error, "Provide at least one channel id or user id."}

      true ->
        {:ok, body, channel_ids, user_ids, guild_id}
    end
  end

  defp split_ids(raw) do
    raw
    |> String.split(~r/[\s,]+/, trim: true)
    |> Enum.reject(&(&1 == ""))
  end

  @impl true
  def render(assigns) do
    ~H"""
    <Layouts.app flash={@flash} current_scope={@current_scope}>
      <div id="operator-dashboard" class="space-y-10">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 class="text-2xl font-semibold tracking-tight">Operator</h1>
            <p class="text-sm text-base-content/70">
              Analytics refresh when new events are written. Memory updates every few seconds.
            </p>
          </div>
          <p class="text-sm text-base-content/60">
            {@guild_total} guilds registered
          </p>
        </div>

        <section class="grid gap-6 lg:grid-cols-2" aria-labelledby="op-telemetry-heading">
          <h2 id="op-telemetry-heading" class="sr-only">Telemetry</h2>
          <div class="rounded-xl border border-base-300 bg-base-100 p-4 shadow-sm">
            <h3 class="text-sm font-medium text-base-content/80">BEAM memory</h3>
            <p class="mt-1 text-2xl font-semibold tabular-nums">{@memory_mb} MB</p>
            <div class="mt-3 h-3 w-full overflow-hidden rounded-full bg-base-300">
              <div
                class="h-full rounded-full bg-primary transition-[width] duration-500 ease-out"
                style={"width: #{@memory_pct}%"}
              >
              </div>
            </div>
          </div>

          <div class="rounded-xl border border-base-300 bg-base-100 p-4 shadow-sm">
            <h3 class="text-sm font-medium text-base-content/80">
              Events per day (last {@chart_days})
            </h3>
            <div class="mt-4 flex h-40 items-end gap-1">
              <%= for {day, n} <- @chart_series do %>
                <% h_pct = if @chart_max > 0, do: round(n / @chart_max * 100), else: 0 %>
                <div class="flex min-w-0 flex-1 flex-col items-center gap-1">
                  <div class="flex h-32 w-full items-end justify-center">
                    <div
                      class="w-full max-w-[2rem] rounded-t bg-accent/80 transition-all"
                      style={"height: #{h_pct}%"}
                      title={"#{day}: #{n}"}
                    >
                    </div>
                  </div>
                  <span class="max-w-full truncate text-[10px] text-base-content/50" title={day}>
                    {String.slice(day, 5..9)}
                  </span>
                </div>
              <% end %>
            </div>
          </div>
        </section>

        <section aria-labelledby="op-events-heading">
          <h2 id="op-events-heading" class="text-lg font-semibold tracking-tight">Event stream</h2>
          <.form
            for={@event_filter_form}
            id="operator-event-filters"
            phx-submit="filter_events"
            class="mt-3 grid gap-3 rounded-xl border border-base-300 bg-base-200/30 p-4 sm:grid-cols-4"
          >
            <.input field={@event_filter_form[:event_type]} type="text" label="Event type" />
            <.input field={@event_filter_form[:guild_id]} type="text" label="Guild id" />
            <.input field={@event_filter_form[:since]} type="date" label="Since (UTC date)" />
            <div class="flex items-end">
              <button type="submit" class="btn btn-secondary btn-sm w-full sm:w-auto">
                Apply filters
              </button>
            </div>
          </.form>
          <div class="mt-3 overflow-x-auto rounded-xl border border-base-300 bg-base-100 shadow-sm">
            <table class="min-w-full border-collapse text-left text-sm">
              <thead class="border-b border-base-300 bg-base-200/50">
                <tr>
                  <th class="px-3 py-2 font-medium">Time</th>
                  <th class="px-3 py-2 font-medium">Event</th>
                  <th class="px-3 py-2 font-medium">Guild</th>
                  <th class="px-3 py-2 font-medium">Channel</th>
                  <th class="px-3 py-2 font-medium">Meta</th>
                </tr>
              </thead>
              <tbody id="operator-events">
                <%= if @events == [] do %>
                  <tr>
                    <td colspan="5" class="px-3 py-8 text-center text-sm text-base-content/60">
                      No analytics events recorded yet.
                    </td>
                  </tr>
                <% end %>
                <%= for ev <- @events do %>
                  <tr
                    id={"operator-event-#{ev.id}"}
                    class="border-b border-base-200/80 odd:bg-base-200/20"
                  >
                    <td class="whitespace-nowrap px-3 py-2 text-xs font-mono">
                      {Calendar.strftime(ev.inserted_at, "%Y-%m-%d %H:%M:%S")}
                    </td>
                    <td class="px-3 py-2 font-medium">{ev.event_type}</td>
                    <td class="px-3 py-2 text-xs">{ev.guild_id || "—"}</td>
                    <td class="px-3 py-2 text-xs">{ev.channel_id || "—"}</td>
                    <td class="max-w-md truncate px-3 py-2 text-xs" title={meta_str(ev.meta)}>
                      {meta_str(ev.meta)}
                    </td>
                  </tr>
                <% end %>
              </tbody>
            </table>
          </div>
        </section>

        <section aria-labelledby="op-guilds-heading">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <h2 id="op-guilds-heading" class="text-lg font-semibold tracking-tight">
              Guild directory
            </h2>
            <div class="flex items-center gap-2">
              <span class="text-xs text-base-content/60">
                Page {@guild_page} of {@guild_total_pages}
              </span>
              <button
                type="button"
                phx-click="guild_page"
                phx-value-dir="prev"
                class="btn btn-ghost btn-xs"
                disabled={@guild_page <= 1}
              >
                Previous
              </button>
              <button
                type="button"
                phx-click="guild_page"
                phx-value-dir="next"
                class="btn btn-ghost btn-xs"
                disabled={@guild_page >= @guild_total_pages}
              >
                Next
              </button>
            </div>
          </div>
          <div class="mt-3 overflow-x-auto rounded-xl border border-base-300 bg-base-100 shadow-sm">
            <table class="min-w-full border-collapse text-left text-sm">
              <thead class="border-b border-base-300 bg-base-200/50">
                <tr>
                  <th class="px-3 py-2 font-medium">#</th>
                  <th class="px-3 py-2 font-medium"></th>
                  <th class="px-3 py-2 font-medium">Name</th>
                  <th class="px-3 py-2 font-medium">Last updated</th>
                  <th class="px-3 py-2 font-medium">Trained</th>
                </tr>
              </thead>
              <tbody id="operator-guilds">
                <%= if @guild_rows == [] do %>
                  <tr>
                    <td colspan="5" class="px-3 py-8 text-center text-sm text-base-content/60">
                      No guilds registered yet.
                    </td>
                  </tr>
                <% end %>
                <%= for row <- @guild_rows do %>
                  <tr class="border-b border-base-200/80 odd:bg-base-200/20">
                    <td class="px-3 py-2 font-mono text-xs">{row.id}</td>
                    <td class="px-3 py-2 text-xs">
                      <%= if row.image_url do %>
                        <img src={row.image_url} alt={row.name} class="w-8 h-8 rounded-full" />
                      <% else %>
                        <span class="w-8 h-8 rounded-full bg-base-300 inline-block"></span>
                      <% end %>
                    </td>
                    <td class="px-3 py-2">{row.name}</td>
                    <td class="whitespace-nowrap px-3 py-2 text-xs">
                      {Calendar.strftime(row.updated_at, "%Y-%m-%d %H:%M")}
                    </td>
                    <td class="px-3 py-2 text-xs">
                      <%= if row.trained_at do %>
                        {Calendar.strftime(row.trained_at, "%Y-%m-%d %H:%M")}
                      <% else %>
                        —
                      <% end %>
                    </td>
                  </tr>
                <% end %>
              </tbody>
            </table>
          </div>
        </section>

        <section
          class="rounded-xl border border-base-300 bg-base-100 p-6 shadow-sm"
          aria-labelledby="op-broadcast-heading"
        >
          <h2 id="op-broadcast-heading" class="text-lg font-semibold tracking-tight">Broadcast</h2>
          <p class="mt-1 text-sm text-base-content/70">
            Sends are queued to the bot runtime over the internal pub/sub bus (not direct from the browser to Discord).
          </p>
          <.form
            for={@broadcast_form}
            id="operator-broadcast-form"
            phx-submit="broadcast"
            class="mt-4 space-y-4"
          >
            <.input
              field={@broadcast_form[:body]}
              type="textarea"
              label="Message"
              required
              phx-debounce="blur"
            />
            <.input
              field={@broadcast_form[:channel_ids_raw]}
              type="text"
              label="Channel ids"
              placeholder="Comma or whitespace separated numeric ids"
            />
            <.input
              field={@broadcast_form[:user_ids_raw]}
              type="text"
              label="User ids (DM, optional)"
              placeholder="Comma or whitespace separated numeric ids"
            />
            <.input
              field={@broadcast_form[:guild_id]}
              type="text"
              label="Guild id (optional, recorded in analytics)"
            />
            <div>
              <button type="submit" class="btn btn-primary">Send broadcast</button>
            </div>
          </.form>
        </section>
      </div>
    </Layouts.app>
    """
  end

  defp meta_str(meta) when is_map(meta) do
    case Jason.encode(meta) do
      {:ok, s} -> s
      _ -> inspect(meta)
    end
  end

  defp meta_str(_), do: ""
end
