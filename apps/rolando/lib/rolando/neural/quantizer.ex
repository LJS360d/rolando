defmodule Rolando.Neural.Quantizer do
  @moduledoc """
  NIF wrapper for ternary quantization of neural network weights.
  Supports both standard and bitnet precision modes.
  """

  @on_load :load_nifs

  @spec load_nifs :: :ok | {:error, atom()}
  defp load_nifs do
    path = :filename.join(:code.priv_dir(:rolando), "nif")
    :erlang.load_nif(path, 0)
  end

  @doc """
  Quantize a float32 weight vector to ternary values (-1, 0, +1).

  ## Arguments
    - weights: List of float32 values
    - threshold: Optional threshold for quantization (defaults to mean absolute value)
    - stochastic: Whether to use stochastic quantization

  ## Returns
    - {values, scale, threshold, zero_ratio}

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

  ## Arguments
    - ternary_values: List of -1, 0, +1 values
    - scale: Scale factor from quantization

  ## Returns
    - List of float32 values

  ## Examples

      iex> Rolando.Neural.Quantizer.dequantize([1, -1, 0, 1], 0.5)
      [0.5, -0.5, 0.0, 0.5]
  """
  @spec dequantize([integer()], float()) :: [float()] | {:error, atom()}
  def dequantize(_ternary_values, _scale), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Quantize a map of weight matrices.

  ## Arguments
    - weights_map: Map of weight matrix name to float array
    - precision_mode: :standard or :bitnet

  ## Returns
    - Map of quantized matrices

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

  ## Arguments
    - quantized_map: Map of quantized matrices

  ## Returns
    - Map of float32 matrices
  """
  @spec dequantize_weights(%{String.t() => {[integer()], float(), float(), float()}}) ::
          %{String.t() => [float()]} | {:error, atom()}
  def dequantize_weights(_quantized_map), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Compute quantization statistics.

  ## Arguments
    - original: Original float32 weights
    - quantized: Ternary quantized values
    - scale: Scale factor used

  ## Returns
    - {compression_ratio, sparsity}
  """
  @spec compute_stats([float()], [integer()], float()) :: {float(), float()} | {:error, atom()}
  def compute_stats(_original, _quantized, _scale), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Compute threshold from weight vector (mean absolute value).
  """
  @spec compute_threshold([float()]) :: float()
  def compute_threshold(weights) when is_list(weights) do
    abs_sum = Enum.reduce(weights, 0.0, fn w, acc -> acc + abs(w) end)
    abs_sum / length(weights)
  end

  @doc """
  Apply standard quantization with default threshold.
  """
  @spec quantize_standard([float()]) :: {[integer()], float()}
  def quantize_standard(weights) when is_list(weights) do
    # Pure Elixir fallback when NIF not loaded
    threshold = compute_threshold(weights)
    abs_weights = Enum.map(weights, &abs/1)
    non_zero = Enum.filter(abs_weights, &(&1 > threshold))
    scale = if non_zero != [], do: Enum.sum(non_zero) / length(non_zero), else: threshold

    values =
      Enum.map(weights, fn
        w when w > threshold -> 1
        w when w < -threshold -> -1
        _ -> 0
      end)

    {values, scale}
  end

  @doc """
  Apply bitnet-style quantization (more aggressive).
  """
  @spec quantize_bitnet([float()]) :: {[integer()], float()}
  def quantize_bitnet(weights) when is_list(weights) do
    # Pure Elixir fallback when NIF not loaded
    threshold = compute_threshold(weights) * 0.7
    abs_weights = Enum.map(weights, &abs/1)
    non_zero = Enum.filter(abs_weights, &(&1 > threshold))
    scale = if non_zero != [], do: Enum.sum(non_zero) / length(non_zero), else: threshold

    values =
      Enum.map(weights, fn
        w when w > threshold -> 1
        w when w < -threshold -> -1
        _ -> 0
      end)

    {values, scale}
  end
end
