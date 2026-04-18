import { ref, computed } from 'vue'
import type { Cluster } from '@/api/clusters'

const clusters  = ref<Cluster[]>([])
const currentID = ref<number | null>(null)

export function useCluster() {
  const current = computed(() => clusters.value.find(c => c.id === currentID.value) ?? null)

  function setClusters(list: Cluster[]) {
    clusters.value = list
    // Auto-select first if nothing selected or selection no longer valid
    if (!currentID.value || !list.find(c => c.id === currentID.value)) {
      currentID.value = list[0]?.id ?? null
    }
  }

  function selectCluster(id: number) {
    currentID.value = id
  }

  return { clusters, currentID, current, setClusters, selectCluster }
}
