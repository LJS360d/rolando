defmodule Rolando.Markov do
  @moduledoc false

  alias Rolando.Markov.Shared

  def store_mod do
    case Application.get_env(:rolando, :markov_store, :ets) do
      :redis -> Rolando.Markov.RedisStore
      _ -> Rolando.Markov.ETSStore
    end
  end

  def reset(guild_id) do
    case store_mod().reset(guild_id) do
      :ok -> :ok
      {:error, _} = e -> e
    end
  end

  def mark_ready(guild_id) do
    case store_mod().mark_ready(guild_id) do
      {:ok, _} -> :ok
      :ok -> :ok
      {:error, _} = e -> e
    end
  end

  def ready?(guild_id), do: store_mod().ready?(guild_id)

  def ingest_texts(guild_id, texts) when is_list(texts) do
    pairs =
      Enum.flat_map(texts, fn t ->
        t
        |> Shared.tokenize_line()
        |> Shared.pairs_from_words()
      end)

    case store_mod().incr_pairs(guild_id, pairs) do
      {:ok, _} -> {:ok, :ingested}
      {:error, _} = e -> e
    end
  end

  def generate(guild_id, seed_text, opts \\ []) do
    max_words = Keyword.get(opts, :max_words, 28)

    if not ready?(guild_id) do
      {:error, :not_trained}
    else
      seed_words =
        case String.trim(to_string(seed_text)) do
          "" -> []
          s -> Shared.tokenize_line(s)
        end

      start_from =
        case List.last(seed_words) do
          nil -> Shared.start_token()
          w -> w
        end

      {new_words, _} = generate_loop(guild_id, start_from, max_words, [])

      out =
        case seed_words ++ new_words do
          [] -> ""
          parts -> Enum.join(parts, " ")
        end

      {:ok, out}
    end
  end

  defp generate_loop(_gid, cur, 0, acc), do: {Enum.reverse(acc), cur}

  defp generate_loop(guild_id, cur, n, acc) do
    stop = Shared.stop_token()

    case store_mod().successors(guild_id, cur) do
      {:ok, counts} when map_size(counts) == 0 ->
        {Enum.reverse(acc), cur}

      {:ok, counts} ->
        case Shared.sample_next(counts) do
          nil ->
            {Enum.reverse(acc), cur}

          ^stop ->
            {Enum.reverse(acc), cur}

          tok ->
            generate_loop(guild_id, tok, n - 1, [tok | acc])
        end

      _ ->
        {Enum.reverse(acc), cur}
    end
  end
end
