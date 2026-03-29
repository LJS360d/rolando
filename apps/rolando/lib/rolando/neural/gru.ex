defmodule Rolando.Neural.GRU do
  @moduledoc """
  NIF wrapper for GRU (Gated Recurrent Unit) neural network operations.
  """

  alias Rolando.Neural.NIF

  @doc """
  Create new GRU weights with Xavier initialization.

  ## Examples

      iex> {:ok, weights} = Rolando.Neural.GRU.create_weights(256, 512)
  """
  @spec create_weights(input_size :: non_neg_integer(), hidden_size :: non_neg_integer()) ::
          binary() | {:error, atom()}
  defdelegate create_weights(input_size, hidden_size), to: NIF

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
  defdelegate forward(input, h_prev, weights), to: NIF

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
  defdelegate forward_sequence(inputs, initial_h, weights), to: NIF

  @doc """
  Get hidden size from weights binary.
  """
  @spec hidden_size(weights :: binary()) :: non_neg_integer()
  defdelegate hidden_size(weights), to: NIF

  @doc """
  Get input size from weights binary.
  """
  @spec input_size(weights :: binary()) :: non_neg_integer()
  defdelegate input_size(weights), to: NIF

  @doc """
  Create a new hidden state vector initialized to zeros.
  """
  @spec zeros(hidden_size :: non_neg_integer()) :: [float()]
  def zeros(hidden_size) do
    for _ <- 1..hidden_size, do: 0.0
  end
end
