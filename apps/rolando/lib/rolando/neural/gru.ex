defmodule Rolando.Neural.GRU do
  @moduledoc """
  NIF wrapper for GRU (Gated Recurrent Unit) neural network operations.
  """

  @on_load :load_nifs

  @spec load_nifs :: :ok | {:error, atom()}
  defp load_nifs do
    path = :filename.join(:code.priv_dir(:rolando), "nif")
    :erlang.load_nif(path, 0)
  end

  @doc """
  Create new GRU weights with Xavier initialization.

  ## Examples

      iex> {:ok, weights} = Rolando.Neural.GRU.create_weights(256, 512)
  """
  @spec create_weights(input_size :: non_neg_integer(), hidden_size :: non_neg_integer()) ::
          binary() | {:error, atom()}
  def create_weights(_input_size, _hidden_size), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Forward pass through GRU for a single timestep.

  ## Arguments
    - input: Input vector (input_size)
    - h_prev: Previous hidden state (hidden_size)
    - weights: Binary containing GRU weights

  ## Returns
    - New hidden state vector
  """
  @spec forward(input :: [float()], h_prev :: [float()], weights :: binary()) ::
          [float()] | {:error, atom()}
  def forward(_input, _h_prev, _weights), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Forward pass through GRU for a sequence.

  ## Arguments
    - inputs: List of input vectors (each input_size)
    - initial_h: Initial hidden state
    - weights: Binary containing GRU weights

  ## Returns
    - List of hidden states for each timestep
  """
  @spec forward_sequence(inputs :: [[float()]], initial_h :: [float()], weights :: binary()) ::
          [[float()]] | {:error, atom()}
  def forward_sequence(_inputs, _initial_h, _weights), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Get hidden size from weights binary.
  """
  @spec hidden_size(weights :: binary()) :: non_neg_integer()
  def hidden_size(_weights), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Get input size from weights binary.
  """
  @spec input_size(weights :: binary()) :: non_neg_integer()
  def input_size(_weights), do: :erlang.nif_error(:nif_not_loaded)

  @doc """
  Create a new hidden state vector initialized to zeros.
  """
  @spec zeros(hidden_size :: non_neg_integer()) :: [float()]
  def zeros(hidden_size) do
    for _ <- 1..hidden_size, do: 0.0
  end
end
