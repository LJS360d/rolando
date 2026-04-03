defmodule Rolando.Neural.Tokenizer do
  @moduledoc """
  NIF wrapper for BPE tokenizer (sentencepiece).
  """

  alias Rolando.Neural.NIF

  @doc """
  Tokenize text into token IDs.

  ## Examples

      iex> Rolando.Neural.Tokenizer.tokenize("hello world")
      [1234, 5678]
  """
  @spec tokenize(String.t()) :: [non_neg_integer()]
  defdelegate tokenize(text),
    to: NIF

  @doc """
  Detokenize token IDs back to text.

  ## Examples

      iex> Rolando.Neural.Tokenizer.detokenize([1234, 5678])
      "hello world"
  """
  @spec detokenize([non_neg_integer()]) :: String.t()
  defdelegate detokenize(token_ids),
    to: NIF

  @doc """
  Load tokenizer model from path.

  ## Examples

      iex> Rolando.Neural.Tokenizer.load_model("priv/models/tokenizer.model")
      :ok
  """
  @spec load_model(String.t()) :: :ok | {:error, atom()}
  defdelegate load_model(model_path),
    to: NIF

  @doc """
  Get vocabulary size.

  ## Examples

      iex> Rolando.Neural.Tokenizer.vocab_size()
      32000
  """
  @spec vocab_size :: non_neg_integer()
  defdelegate vocab_size,
    to: NIF

  @doc """
  Get token ID for a given token string.

  ## Examples

      iex> Rolando.Neural.Tokenizer.get_token_id("hello")
      1234
  """
  @spec get_token_id(String.t()) :: non_neg_integer()
  defdelegate get_token_id(token), to: NIF

  @doc """
  UTF-8 byte-level token IDs (0–255 per byte). Used for language model training and generation.
  """
  @spec tokenize_bytes(String.t()) :: [non_neg_integer()]
  defdelegate tokenize_bytes(text), to: NIF

  @spec detokenize_bytes([non_neg_integer()]) :: String.t()
  defdelegate detokenize_bytes(ids), to: NIF

  @spec byte_vocab_size :: non_neg_integer()
  defdelegate byte_vocab_size(), to: NIF, as: :tokenizer_byte_vocab_size
end
