import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import * as tasksApi from '../api/tasks'
import type { TaskStatus } from '../api/types'

export function useTasks(status?: TaskStatus) {
  return useQuery({
    queryKey: ['tasks', status],
    queryFn: () => tasksApi.list(status),
    refetchInterval: 5_000,
  })
}

export function useTask(id: string) {
  return useQuery({
    queryKey: ['task', id],
    queryFn: () => tasksApi.get(id),
    enabled: !!id,
    refetchInterval: (query) => {
      const s = query.state.data?.status
      if (s === 'queued' || s === 'processing' || s === 'retrying') return 2_000
      return false
    },
  })
}

export function useCancelTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => tasksApi.cancel(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useEnqueue() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: tasksApi.EnqueueRequest) => tasksApi.enqueue(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}
