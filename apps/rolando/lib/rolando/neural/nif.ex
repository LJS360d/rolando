defmodule Rolando.Neural.NIF do
  @on_load :load_nifs

  @spec load_nifs :: :ok | {:error, atom()}
  defp load_nifs do
    path = :filename.join(:code.priv_dir(:rolando), "nif/rolando_nif")

    :erlang.load_nif(path, 0)
  end

  # GRU - names match Rust exports

  @doc """
  Create new GRU weights with Xavier initialization.

  ## Examples

      iex> {:ok, weights} = Rolando.Neural.GRU.gru_create_weights(256, 512)
  """
  @spec gru_create_weights(input_size :: non_neg_integer(), hidden_size :: non_neg_integer()) ::
          binary() | {:error, atom()}
  def gru_create_weights(_input_size, _hidden_size), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Forward pass through GRU for a single timestep.

  ## Arguments
    - input: Input vector (input_size)
    - h_prev: Previous hidden state (hidden_size)
    - weights: Binary containing GRU weights

  ## Returns
    - New hidden state vector
  """
  @spec gru_forward(input :: [float()], h_prev :: [float()], weights :: binary()) ::
          [float()] | {:error, atom()}
  def gru_forward(_input, _h_prev, _weights), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Forward pass through GRU for a sequence.

  ## Arguments
    - inputs: List of input vectors (each input_size)
    - initial_h: Initial hidden state
    - weights: Binary containing GRU weights

  ## Returns
    - List of hidden states for each timestep
  """
  @spec gru_forward_sequence(inputs :: [[float()]], initial_h :: [float()], weights :: binary()) ::
          [[float()]] | {:error, atom()}
  def gru_forward_sequence(_inputs, _initial_h, _weights), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Get hidden size from weights binary.
  """
  @spec gru_hidden_size(weights :: binary()) :: non_neg_integer()
  def gru_hidden_size(_weights), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Get input size from weights binary.
  """
  @spec gru_input_size(weights :: binary()) :: non_neg_integer()
  def gru_input_size(_weights), do: :erlang.nif_error(:nif_not_loaded)

  # Quantizer

  @doc """
  Quantize a float32 weight vector to ternary values (-1, 0, +1).

  ## Examples

      iex> {values, scale, threshold, _} = Rolando.Neural.Quantizer.quantize([0.5, -0.3, 0.1, 0.8])
      iex> values
      [1, -1, 0, 1]
  """
  @spec quantize([float()], threshold :: float() | nil, stochastic :: boolean() | nil) ::
          {[integer()], float(), float(), float()} | {:error, atom()}
  def quantize(_weights, _threshold \\ nil, _stochastic \\ false),
    do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Dequantize ternary values back to float32.

  ## Examples

      iex> Rolando.Neural.Quantizer.dequantize([1, -1, 0, 1], 0.5)
      [0.5, -0.5, 0.0, 0.5]
  """
  @spec dequantize([integer()], float()) :: [float()] | {:error, atom()}
  def dequantize(_ternary_values, _scale), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Quantize a map of weight matrices.

  ## Examples

      iex> weights = %{"w_z" => [0.5, -0.3], "w_r" => [0.1, 0.8]}
      iex> Rolando.Neural.Quantizer.quantize_weights(weights, :standard)
      %{"w_z" => {[1, -1], 0.4, ...}, "w_r" => {...}}
  """
  @spec quantize_weights(%{String.t() => [float()]}, :standard | :bitnet) ::
          %{String.t() => {[integer()], float(), float(), float()}} | {:error, atom()}
  def quantize_weights(_weights_map, _precision_mode), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Dequantize a map of ternary matrices back to float32.
  """
  @spec dequantize_weights(%{String.t() => {[integer()], float(), float(), float()}}) ::
          %{String.t() => [float()]} | {:error, atom()}
  def dequantize_weights(_quantized_map), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Compute quantization statistics.
  """
  @spec compute_stats([float()], [integer()], float()) :: {float(), float()} | {:error, atom()}
  def compute_stats(_original, _quantized, _scale), do: :erlang.nif_error(:nif_not_loaded)

  # Tokenizer

  @doc """
  Tokenize text into token IDs.

  ## Examples

      iex> Rolando.Neural.Tokenizer.tokenize("hello world")
      [1234, 5678]
  """
  @spec tokenize(String.t()) :: [non_neg_integer()]
  def tokenize(_text),
    do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Detokenize token IDs back to text.

  ## Examples

      iex> Rolando.Neural.Tokenizer.detokenize([1234, 5678])
      "detokenized"
  """
  @spec detokenize([non_neg_integer()]) :: String.t()
  def detokenize(_token_ids),
    do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Load tokenizer model from path.

  ## Examples

      iex> Rolando.Neural.Tokenizer.load_model("priv/models/tokenizer.model")
      :ok
  """
  @spec load_model(String.t()) :: :ok | {:error, atom()}
  def load_model(_model_path),
    do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Get vocabulary size.

  ## Examples

      iex> Rolando.Neural.Tokenizer.vocab_size()
      32000
  """
  @spec vocab_size :: non_neg_integer()
  def vocab_size,
    do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Get token ID for a given token string.

  ## Examples

      iex> Rolando.Neural.Tokenizer.get_token_id("hello")
      1234
  """
  @spec get_token_id(String.t()) :: non_neg_integer()
  def get_token_id(_token),
    do: :erlang.nif_error(:nif_not_loaded)
end
