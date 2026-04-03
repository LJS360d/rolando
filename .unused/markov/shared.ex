defmodule Rolando.Markov.Shared do
  @moduledoc false

  @start "__START__"
  @stop "__END__"

  def start_token, do: @start
  def stop_token, do: @stop

  def from_hash(word) do
    :crypto.hash(:sha256, word)
    |> Base.encode16(case: :lower)
    |> binary_part(0, 32)
  end

  def field_token(word) when is_binary(word) do
    Base.url_encode64(word, padding: false)
  end

  def field_decode(field) when is_binary(field) do
    case Base.url_decode64(field, padding: :optional) do
      {:ok, bin} -> bin
      :error -> field
    end
  end

  def tokenize_line(text) when is_binary(text) do
    text
    |> String.downcase()
    |> String.replace(~r/[^\p{L}\p{N}\s]/u, " ")
    |> String.split()
    |> Enum.filter(&(&1 != ""))
  end

  def pairs_from_words(words) when is_list(words) do
    words = [@start | words] ++ [@stop]

    words
    |> Enum.chunk_every(2, 1, :discard)
    |> Enum.map(fn [a, b] -> {a, b} end)
  end

  def sample_next(%{} = counts) when map_size(counts) == 0, do: nil

  def sample_next(counts) do
    counts = decode_successors(counts)
    total = Enum.reduce(counts, 0, fn {_, c}, acc -> acc + c end)

    if total <= 0 do
      nil
    else
      r = :rand.uniform(total)

      Enum.reduce_while(counts, r, fn {tok, weight}, left ->
        if left <= weight, do: {:halt, tok}, else: {:cont, left - weight}
      end)
    end
  end

  def decode_successors(counts) when is_map(counts) do
    Map.new(counts, fn {f, c} -> {field_decode(f), c} end)
  end
end
