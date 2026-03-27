defmodule Rolando.Markov do
  @moduledoc false

  defstruct [:ngram_size, :state, :message_counter]

  def new(attrs \\ []) do
    n = Keyword.get(attrs, :ngram_size, 2)
    n = if n < 2, do: 2, else: n

    %__MODULE__{
      ngram_size: n,
      state: %{},
      message_counter: 0
    }
  end

  def provide_data(%__MODULE__{} = m, messages) when is_list(messages) do
    Enum.reduce(messages, m, fn line, acc -> update_state(acc, line) end)
  end

  def update_state(%__MODULE__{} = m, message) when is_binary(message) do
    if String.starts_with?(message, "https://") or String.starts_with?(message, "http://") do
      m
    else
      do_update_state(m, message)
    end
  end

  defp do_update_state(%__MODULE__{ngram_size: n} = m, message) do
    tokens = tokenize(message)
    len = length(tokens)

    if len < n do
      m
    else
      max_i = len - n

      state =
        Enum.reduce(0..max_i, m.state, fn i, st ->
          prefix_tokens = Enum.slice(tokens, i, n - 1)
          next_word = Enum.at(tokens, i + n - 1)
          prefix_key = Enum.join(prefix_tokens, " ")
          inner = Map.get(st, prefix_key, %{})
          inner = Map.update(inner, next_word, 1, &(&1 + 1))
          Map.put(st, prefix_key, inner)
        end)

      %{m | state: state, message_counter: m.message_counter + 1}
    end
  end

  def tokenize(text) do
    text
    |> String.split()
    |> Enum.filter(&(&1 != ""))
  end

  def apply_batch(%__MODULE__{} = m, lines) when is_list(lines) do
    provide_data(m, lines)
  end

  def to_json_map(%__MODULE__{ngram_size: n, state: st, message_counter: c}) do
    %{
      "v" => 1,
      "ngram_size" => n,
      "message_counter" => c,
      "state" => st
    }
  end

  def from_json_map(map) when is_map(map) do
    n = Map.get(map, "ngram_size", 2)
    c = Map.get(map, "message_counter", 0)
    raw = Map.get(map, "state", %{})

    st =
      raw
      |> Enum.map(fn {k, v} ->
        inner =
          v
          |> Enum.map(fn
            {ik, iv} when is_integer(iv) -> {to_string(ik), iv}
            {ik, iv} -> {to_string(ik), iv}
          end)
          |> Map.new()

        {to_string(k), inner}
      end)
      |> Map.new()

    %__MODULE__{ngram_size: max(2, n), state: st, message_counter: c}
  end

  def serialize(%__MODULE__{} = m) do
    Jason.encode!(to_json_map(m))
  end

  def deserialize(nil), do: new()

  def deserialize(json) when is_binary(json) and json != "" do
    case Jason.decode(json) do
      {:ok, map} -> from_json_map(map)
      {:error, _} -> new()
    end
  end

  def deserialize(_), do: new()
end
