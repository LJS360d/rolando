# Script for populating the database. You can run it as:
#
#     mix run priv/repo/seeds.exs
#
# Inside the script, you can read and write to any of your
# repositories directly:
#
#     Rolando.Repo.insert!(%Rolando.SomeSchema{})
#
# We recommend using the bang functions (`insert!`, `update!`
# and so on) as they will fail if something goes wrong.

defmodule Rolando.Seeds do
  @moduledoc false

  alias Rolando.Repo
  require Logger

  # Fixed seed for reproducible Xavier uniform initialization
  # This ensures all nodes in a cluster start with the same weights
  @seed {0x12_34_56_78, 0xAB_CD_EF_00, 0x12_34_56_78}

  # Model dimensions - these should match the tier configuration
  # Standard tier: embedding_dim=256, hidden_dim=512, vocab_size=32000
  @vocab_size 32_000
  @embedding_dim 256
  @hidden_dim 512

  def run do
    IO.puts("Seeding shared weights with Xavier uniform initialization...")

    # Check if already initialized
    case Rolando.Contexts.SharedWeights.exists?() do
      true ->
        IO.puts("Shared weights already initialized, skipping.")

      false ->
        IO.puts("Generating Xavier uniform initialized weights...")
        embedding_data = generate_xavier_uniform(@vocab_size, @embedding_dim)
        projection_data = generate_xavier_uniform(@embedding_dim, @hidden_dim)

        case Rolando.Contexts.SharedWeights.initialize(embedding_data, projection_data, :standard) do
          {:ok, shared_weights} ->
            IO.puts("Shared weights initialized successfully!")
            IO.puts("  - ID: #{shared_weights.id}")
            IO.puts("  - Tier: #{shared_weights.tier}")
            IO.puts("  - Embedding shape: #{@vocab_size}x#{@embedding_dim}")
            IO.puts("  - Projection shape: #{@embedding_dim}x#{@hidden_dim}")

          {:error, changeset} ->
            IO.puts("Failed to initialize shared weights:")
            IO.inspect(changeset.errors)
        end
    end
  end

  # Xavier/Glorot uniform initialization
  # W ~ Uniform(-limit, limit) where limit = sqrt(6 / (fan_in + fan_out))
  defp generate_xavier_uniform(fan_in, fan_out) do
    limit = :math.sqrt(6.0 / (fan_in + fan_out))

    # Use the fixed seed for reproducibility
    :rand.seed(:exsplus, @seed)

    # Generate random values in range [-limit, limit]
    # For binary serialization, we use float32
    values =
      for _ <- 1..(fan_in * fan_out) do
        # Generate a random float in [0, 1) and scale to [-limit, limit]
        random_float = :rand.uniform()
        scaled = (random_float * 2.0 - 1.0) * limit

        # Convert to float32 binary (little-endian)
        <<scaled::float-size(32)-little>>
      end

    # Concatenate all float32 values into a single binary
    :binary.list_to_bin(values)
  end

  # Start the Ecto repo if not already running
  defp start_repo_if_not_running do
    case Repo.start_link() do
      {:ok, _} -> {:ok, :started}
      {:error, {:already_started, _}} -> {:ok, :already_running}
      error -> error
    end
  end

  def start_repo do
    Application.ensure_all_started(:rolando)
    start_repo_if_not_running()
  end
end

# Run the seeds
Rolando.Seeds.start_repo()
Rolando.Seeds.run()
