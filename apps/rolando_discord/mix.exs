defmodule RolandoDiscord.MixProject do
  use Mix.Project

  def project do
    [
      app: :rolando_discord,
      version: "0.1.0",
      build_path: "../../_build",
      config_path: "../../config/config.exs",
      deps_path: "../../deps",
      lockfile: "../../mix.lock",
      elixir: "~> 1.19",
      start_permanent: Mix.env() == :prod,
      deps: deps()
    ]
  end

  # Run "mix help compile.app" to learn about applications.
  def application do
    [
      extra_applications: [:logger, :nostrum],
      mod: {RolandoDiscord.Application, []}
    ]
  end

  # Run "mix help deps" to learn about dependencies.
  defp deps do
    [
      # {:dep_from_hexpm, "~> 0.3.0"},
      # {:dep_from_git, git: "https://github.com/elixir-lang/my_dep.git", tag: "0.1.0"},
      # {:sibling_app_in_umbrella, in_umbrella: true},
      {:nostrum, "~> 0.10"},
      {:horde, "~> 0.10.0"},
      {:rolando, in_umbrella: true}
    ]
  end

  def aliases do
    [
      setup: [""]
    ]
  end
end
