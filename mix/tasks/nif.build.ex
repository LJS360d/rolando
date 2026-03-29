# mix/tasks/nif.build.ex (in root)
defmodule Mix.Tasks.Nif.Build do
  use Mix.Task

  @shortdoc "Build Rustler NIF"

  def run(_args) do
    Mix.Project.in_project(:rolando, "apps/rolando", fn _module ->
      Mix.Task.run("nif.build")
    end)
  end
end
