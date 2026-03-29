defmodule Rolando.Neural.Tokenizer do
  @moduledoc """
  NIF wrapper for BPE tokenizer (sentencepiece).
  """

  @on_load :load_nifs

  @spec load_nifs :: :ok
  defp load_nifs do
    :erlang.load_nif(Application.app_dir(:rolando, "priv/nif"), 0)
  end

  @doc """
  Tokenize text into token IDs.

  ## Examples

      iex> Rolando.Neural.Tokenizer.tokenize("hello world")
      [1234, 5678]
  """
  @spec tokenize(String.t()) :: [non_neg_integer()]
  def tokenize(_text), do: raise("NIF not loaded")

  @doc """
  Detokenize token IDs back to text.

  ## Examples

      iex> Rolando.Neural.Tokenizer.detokenize([1234, 5678])
      "hello world"
  """
  @spec detokenize([non_neg_integer()]) :: String.t()
  def detokenize(_token_ids), do: raise("NIF not loaded")

  @doc """
  Load tokenizer model from path.

  ## Examples

      iex> Rolando.Neural.Tokenizer.load_model("priv/models/tokenizer.model")
      :ok
  """
  @spec load_model(String.t()) :: :ok | {:error, atom()}
  def load_model(_model_path), do: raise("NIF not loaded")

  @doc """
  Get vocabulary size.

  ## Examples

      iex> Rolando.Neural.Tokenizer.vocab_size()
      32000
  """
  @spec vocab_size :: non_neg_integer()
  def vocab_size, do: raise("NIF not loaded")

  @doc """
  Get token ID for a given token string.

  ## Examples

      iex> Rolando.Neural.Tokenizer.get_token_id("hello")
      1234
  """
  @spec get_token_id(String.t()) :: non_neg_integer()
  def get_token_id(_token), do: raise("NIF not loaded")
end
