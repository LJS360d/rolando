defmodule Rolando.Training.PoolWorker do
  @moduledoc """
  GenServer worker for CPU-intensive training steps.
  Manages a pool of worker processes for parallel training.
  """
  use GenServer

  # Client API

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  def submit_training_job(job) do
    GenServer.call(__MODULE__, {:submit_job, job})
  end

  def get_pool_status do
    GenServer.call(__MODULE__, :get_status)
  end

  # Server Callbacks

  def init(opts) do
    pool_size = Keyword.get(opts, :pool_size, 4)
    state = %{pool_size: pool_size, jobs_in_progress: 0, completed_jobs: 0}
    {:ok, state}
  end

  def handle_call({:submit_job, _job}, _from, state) do
    # TODO: Implement actual training job processing
    # For now, just acknowledge receipt
    new_state = %{state | jobs_in_progress: state.jobs_in_progress + 1}
    {:reply, {:ok, :queued}, new_state}
  end

  def handle_call(:get_status, _from, state) do
    status = %{
      pool_size: state.pool_size,
      jobs_in_progress: state.jobs_in_progress,
      completed_jobs: state.completed_jobs
    }

    {:reply, status, state}
  end
end
