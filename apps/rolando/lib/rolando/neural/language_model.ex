defmodule Rolando.Neural.LanguageModel do
  @moduledoc false

  alias Rolando.Neural.NIF

  @magic "RLM1"

  def language_model?(data) do
    try do
      binary_part(binaryize!(data), 0, 4) == @magic
    rescue
      _ -> false
    end
  end

  def create(opts \\ []) do
    vb = Keyword.fetch!(opts, :vocab_buckets)
    e = Keyword.fetch!(opts, :embed_dim)
    h = Keyword.fetch!(opts, :hidden_dim)
    NIF.language_model_create(vb, e, h) |> binaryize!()
  end

  def train_chunk(weights, sequences, opts \\ []) do
    lr = Keyword.get(opts, :lr, 0.05)
    epochs = Keyword.get(opts, :epochs, 1)
    max_seq_len = Keyword.get(opts, :max_seq_len, 128)
    max_norm = Keyword.get(opts, :gradient_clip, 5.0)

    n = length(sequences)
    lengths = Enum.map(sequences, &length/1)
    flat = Enum.flat_map(sequences, & &1)
    packed = [n | lengths ++ flat]

    hyper_bin =
      <<lr::float-little-32, epochs::little-64, max_seq_len::little-64, max_norm::float-little-32>>

    {bin, loss} = NIF.language_model_train(binaryize!(weights), packed, hyper_bin)

    {:ok, binaryize!(bin), loss}
  end

  def dims(weights) do
    try do
      {vb, e, h} = NIF.language_model_dims(binaryize!(weights))
      {:ok, {vb, e, h}}
    rescue
      _ -> {:error, :invalid_model}
    end
  end

  def hidden_zeros(weights) do
    case dims(weights) do
      {:ok, {_vb, _e, h}} -> for(_ <- 1..h, do: 0.0)
      {:error, _} -> []
    end
  end

  def step(weights, token_id, h) do
    try do
      {logits, h2} = NIF.language_model_step(binaryize!(weights), token_id, h)
      {:ok, logits, h2}
    rescue
      _ -> {:error, :step_failed}
    end
  end

  def generate(weights, seed_text, opts \\ []) do
    max_tokens = Keyword.get(opts, :max_tokens, 24)
    temperature = Keyword.get(opts, :temperature, 0.9)

    case Keyword.get(opts, :codebook) do
      codebook when is_list(codebook) ->
        generate_word_model(weights, seed_text, max_tokens, temperature, codebook)

      _ ->
        generate_byte_model(weights, seed_text, max_tokens, temperature)
    end
  end

  def generate_bytes(weights, seed_text, opts \\ []) do
    max_tokens = Keyword.get(opts, :max_tokens, 24)
    temperature = Keyword.get(opts, :temperature, 0.9)
    generate_byte_model(weights, seed_text, max_tokens, temperature)
  end

  defp generate_word_model(weights, seed_text, max_tokens, temperature, codebook) do
    seed_ids =
      seed_text
      |> String.trim()
      |> then(fn
        "" ->
          case Rolando.Neural.Tokenizer.tokenize(".") do
            [] -> [0]
            xs -> Enum.take(xs, -128)
          end

        s ->
          Rolando.Neural.Tokenizer.tokenize(s) |> Enum.take(-128)
      end)

    h = hidden_zeros(weights)

    with {:ok, vb, _e, hid} <- dims_tuple(weights),
         true <- length(h) == hid,
         true <- length(codebook) == vb,
         {:ok, logits, h} <- run_seed(weights, seed_ids, h) do
      {gen, _} =
        Enum.reduce(1..max_tokens, {[], {logits, h}}, fn _, {acc, {lg, hh}} ->
          tid = sample_token_id(lg, temperature, 0)

          case step(weights, tid, hh) do
            {:ok, lg2, h2} -> {[tid | acc], {lg2, h2}}
            {:error, _} -> {acc, {lg, hh}}
          end
        end)

      text =
        gen
        |> Enum.reverse()
        |> Enum.map(&Enum.at(codebook, &1, ""))
        |> Enum.reject(&(&1 == ""))
        |> Enum.join(" ")
        |> scrub_word_output()

      {:ok, text}
    else
      false -> {:error, :bad_state}
      {:error, r} -> {:error, r}
      _ -> {:error, :bad_state}
    end
  end

  defp generate_byte_model(weights, seed_text, max_tokens, temperature) do
    seed_ids =
      seed_text
      |> String.trim()
      |> then(fn
        "" -> [32]
        s -> Rolando.Neural.Tokenizer.tokenize_bytes(s)
      end)
      |> Enum.take(-128)

    h = hidden_zeros(weights)

    with {:ok, _vb, _e, hid} <- dims_tuple(weights),
         true <- length(h) == hid,
         {:ok, logits, h} <- run_seed(weights, seed_ids, h) do
      {gen, _} =
        Enum.reduce(1..max_tokens, {[], {logits, h}}, fn _, {acc, {lg, hh}} ->
          tid = sample_token_id(lg, temperature, 32)

          case step(weights, tid, hh) do
            {:ok, lg2, h2} -> {[tid | acc], {lg2, h2}}
            {:error, _} -> {acc, {lg, hh}}
          end
        end)

      text =
        gen
        |> Enum.reverse()
        |> tokens_to_utf8_string()

      {:ok, scrub_byte_output(text)}
    else
      false -> {:error, :bad_state}
      {:error, r} -> {:error, r}
      _ -> {:error, :bad_state}
    end
  end

  defp run_seed(weights, seed_ids, h) do
    Enum.reduce_while(seed_ids, {:ok, nil, h}, fn tid, {:ok, _, hh} ->
      case step(weights, tid, hh) do
        {:ok, lg, h2} -> {:cont, {:ok, lg, h2}}
        {:error, r} -> {:halt, {:error, r}}
      end
    end)
    |> case do
      {:ok, logits, h} -> {:ok, logits, h}
      other -> other
    end
  end

  defp binaryize!(data) do
    cond do
      is_binary(data) -> data
      is_list(data) -> :binary.list_to_bin(data)
      true -> raise ArgumentError, "expected binary or charlist from NIF"
    end
  end

  defp dims_tuple(weights) do
    case dims(weights) do
      {:ok, {vb, e, h}} -> {:ok, vb, e, h}
      e -> e
    end
  end

  defp sample_token_id(logits, temp, default_id)
       when is_list(logits) and temp > 0 do
    t = max(temp, 1.0e-6)
    mx = Enum.max(logits)

    probs =
      logits
      |> Enum.map(fn x -> :math.exp((x - mx) / t) end)

    s = Enum.sum(probs)

    if s <= 1.0e-12 do
      default_id
    else
      probs = Enum.map(probs, fn p -> p / s end)
      r = :rand.uniform(1_000_000) / 1_000_000
      sample_from_probs(probs, r, 0.0, 0)
    end
  end

  defp sample_from_probs([p | rest], r, acc, idx) do
    acc = acc + p

    if r <= acc do
      idx
    else
      sample_from_probs(rest, r, acc, idx + 1)
    end
  end

  defp sample_from_probs([], _r, _acc, idx), do: idx

  defp tokens_to_utf8_string(tokens) do
    bytes =
      tokens
      |> Enum.map(&rem(&1, 256))
      |> :binary.list_to_bin()

    cond do
      String.valid?(bytes) ->
        bytes

      byte_size(bytes) == 0 ->
        ""

      true ->
        :binary.bin_to_list(bytes) |> List.to_string()
    end
  end

  defp scrub_byte_output(text) when is_binary(text) do
    text
    |> String.replace("\uFFFD", "")
    |> String.replace(~r/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/u, "")
    |> String.trim()
  end

  defp scrub_word_output(text) when is_binary(text) do
    text
    |> String.replace(~r/\s+/, " ")
    |> String.trim()
  end
end
