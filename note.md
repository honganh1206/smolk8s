Goal: A declarative system where a user declares the desired state of a task.

What problems does an orchestrator solve?

- Deploy apps on bare metal servers -> VMs resolve this
- Each app has its own hardware requirements, unique deployment process and tooling
- VMs are coupled with one OS even though many VMs can run on one physical machine

Container = process and resource isolation -> Each app thinks that it is the only one running on the OS with all the resources.

Orchestrator is similar to a CPU scheduler

## Components

Task - Job - (Scheduler - Manager - Worker) Must haves - Cluster - CLI

Task = A process running on a single machine. A task can run an instance of NGINX or a RESTful API app

A task specify: Amount of memory/CPU/disk, restart policy, name of container image

Job = Aggregation of tasks (a logical grouping like DAG?)

> Job in k8s: A workload that runs from start to finish. Can be of different resource types like Deployment/ReplicaSet, StatefulSet, DaemonSet, Job.

Scheduler decides what machine can best host the tasks defined by a job. It uses an algorithm to determine the best candidate machine to run the task? Like round-robin or E-PVM (Or Paxos for Google's Borg)
Three phases: Feasibility (Possible to schedule a task onto a worker) -> Scoring (Give each worker a score) -> Picking

Manager is the brain of the system and it does:
1. Invoke the scheduler to schedule tasks on worker machines 
2. Collect metrics from workers for the scheduling process

Workers run the tasks as (Docker?) containers and retry if the tasks fail, and make metrics of tasks

Cluster is the logical grouping of all the components. A cluster can be built from multiple machines.

## Examples

k8s has control plane (Manager) and kubelet (Worker). Jobs and tasks are combined as k8s objects.

Nomad: Manager = Server. Worker = Client. Client communicate with Server via RPC. Servers replicate + forward data between each other. There is one leader server (Raft?)

## Flow

User sends a job -> Manager calculates where it should place a task with the Scheduler -> Manager sends tasks to Workers and pull metrics from the Workers' APIs.

Both Manager and Workers keep track of the work with the Task Storage layer

Multiple instances of Worker and one instance of Manager, since Workers don't care about each other's work, but Manager oversees the worker and maintains the cluster's state -> Challenging to sync state between instances. 

## Task

A task first submitted (Pending) -> Manager figures out where to run the task (Scheduled) -> Selected machine starts the task (Running) -> Task is completed (Completed)

A task is like a step when making a pizza. The context of a task is a Docker container, which provides necessary resources like CPU, memory and disk.

## Worker

The Workers will call the Docker APIs to interact with the Tasks (under Docker's containers)

A worker represents both a physical/virtual machine, and the worker component of the orchestration system

## Metrics

Collect via `/proc`

## Manager

Isolate administrative concerns from execution concerns => Separation of concerns like handling requests, assigning tasks, keeping track of tasks and worker state, restarting failed tasks, etc.

Control plane controls how data moves from A to B e.g., BGP, OSPF. Data plane does the actual work of moving the data around.

## State machine

Manager receives tasks as Pending -> Hand out tasks to workers (Scheduled) -> Workers run tasks with Docker client (Running) -> Task done running (Completed) or failed (Failed) -> Manager compares task state in DB with task state returned from workers -> Sync task state to state of response
