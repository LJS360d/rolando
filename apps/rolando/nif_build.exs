#!/usr/bin/env elixir
# NIF Build Script for Rolando
# This script builds the Rust NIF and copies it to the correct location

defmodule Rolando.NIFBuild do
  @moduledoc """
  Build script for the Rolando NIF library.
  Compiles the Rust code and copies the shared library to priv/nif/
  """

  @nif_name "rolando_nif"
  @native_path "native"
  @target_path "#{@native_path}/target"
  @priv_nif_path "priv/nif"

  def main(args) do
    IO.puts("🔧 Building Rolando NIF...")

    # Parse command line arguments
    {opts, _} = OptionParser.parse!(args, strict: [release: :boolean, clean: :boolean])
    release? = Keyword.get(opts, :release, false)
    clean? = Keyword.get(opts, :clean, false)

    if clean?, do: clean_build()

    # Build the NIF
    build_nif(release?)

    # Copy to priv/nif
    copy_to_priv()

    IO.puts("✅ NIF build completed successfully!")
  end

  defp clean_build do
    IO.puts("🧹 Cleaning build artifacts...")
    System.cmd("cargo", ["clean"], cd: @native_path)
    File.rm_rf!(@priv_nif_path)
  end

  defp build_nif(release?) do
    IO.puts(if release?, do: "🏗️  Building release NIF...", else: "🏗️  Building debug NIF...")

    # Build the Rust NIF
    profile = if release?, do: ["--release"], else: []
    args = ["build"] ++ profile

    case System.cmd("cargo", args, cd: @native_path, into: IO.stream()) do
      {_, 0} ->
        :ok

      {error, _} ->
        IO.puts(:stderr, "❌ NIF build failed: #{error}")
        System.halt(1)
    end
  end

  defp copy_to_priv do
    IO.puts("📦 Copying NIF to priv/nif...")

    # Ensure priv/nif directory exists
    File.mkdir_p!(@priv_nif_path)

    # Find the built library
    lib_pattern = "#{@target_path}/**/lib#{@nif_name}.so"

    case Path.wildcard(lib_pattern) do
      [lib_path] ->
        dest_path = Path.join(@priv_nif_path, "#{@nif_name}.so")
        File.cp!(lib_path, dest_path)
        IO.puts("📁 Copied #{lib_path} -> #{dest_path}")

      [] ->
        IO.puts(:stderr, "❌ Could not find built NIF library")
        System.halt(1)

      multiple ->
        IO.puts(:stderr, "❌ Found multiple NIF libraries: #{inspect(multiple)}")
        System.halt(1)
    end
  end
end

# Run the build script
Rolando.NIFBuild.main(System.argv())
