defmodule Rolando.MarkovTest do
  use ExUnit.Case, async: true

  alias Rolando.Markov

  test "update_state builds n-gram transitions" do
    m =
      Markov.new(ngram_size: 2)
      |> Markov.update_state("hello world here")

    assert m.state["hello"]["world"] == 1
    assert m.state["world"]["here"] == 1
  end

  test "serialize roundtrip" do
    m =
      Markov.new(ngram_size: 2)
      |> Markov.update_state("a b c d")

    json = Markov.serialize(m)
    m2 = Markov.deserialize(json)
    assert m2.ngram_size == 2
    assert m2.state == m.state
  end
end
