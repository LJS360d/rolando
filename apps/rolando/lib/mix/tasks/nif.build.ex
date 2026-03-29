defmodule Mix.Tasks.Nif.Build do
  use Mix.Task
  @shortdoc "Build Rustler NIF and copy to priv/"
  def run(_args) do
    app_root =
      Mix.Project.build_path() |> Path.dirname() |> Path.dirname() |> Path.join("apps/rolando")

    nif_dir = Path.join(app_root, "native")
    priv_dir = Path.join(app_root, "priv/nif")

    Mix.shell().info("Building Rust NIF...")

    case System.cmd("cargo", ["build", "--release"], cd: nif_dir) do
      {_output, 0} ->
        Mix.shell().info("✓ Cargo build successful")

      {error, code} ->
        Mix.raise("Cargo build failed (exit code #{code}):\n#{error}")
    end

    File.mkdir_p!(priv_dir)

    src = Path.join(nif_dir, "target/release/librolando_nif.so")
    dst = Path.join(priv_dir, "rolando_nif.so")

    if File.exists?(src) do
      File.cp!(src, dst)
      Mix.shell().info("✓ Copied #{src} → #{dst}")
    else
      Mix.raise("Could not find built NIF at #{src}")
    end
  end
end
