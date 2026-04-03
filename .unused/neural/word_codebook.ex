defmodule Rolando.Neural.WordCodebook do
  @moduledoc false

  def encode(words) when is_list(words) do
    {:codebook_v1, words} |> :erlang.term_to_binary()
  end

  def decode(nil), do: nil
  def decode(<<>>), do: nil

  def decode(bin) when is_binary(bin) do
    case :erlang.binary_to_term(bin, [:safe]) do
      {:codebook_v1, list} when is_list(list) -> list
      _ -> nil
    end
  rescue
    ArgumentError -> nil
  end
end
