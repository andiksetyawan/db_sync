<template>
  <div class="container mx-auto px-4 py-8 max-w-7xl">
    <div class="mb-8">
      <h1 class="text-4xl font-bold text-base-content mb-2">
        🔄 Database Sync Scheduler
      </h1>
      <p class="text-base-content/70">
        Monitor and control your database synchronization service with FK-aware ordering
      </p>
    </div>

    <div class="card bg-base-100 shadow-xl mb-6">
      <div class="card-body">
        <h2 class="card-title text-2xl mb-4">Control Panel</h2>

        <div class="flex flex-wrap gap-4 items-center">
          <button
            @click="startSync"
            :disabled="loading || syncStatus?.isRunning"
            class="btn btn-success btn-lg gap-2"
            :class="{ 'loading': loading }"
          >
            <svg v-if="!loading" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {{ syncStatus?.isRunning ? 'Running' : 'Start Sync' }}
          </button>

          <button
            @click="stopSync"
            :disabled="loading || !syncStatus?.isRunning"
            class="btn btn-error btn-lg gap-2"
            :class="{ 'loading': loading }"
          >
            <svg v-if="!loading" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
            </svg>
            Stop Sync
          </button>

          <button
            @click="triggerSchemaSync"
            :disabled="loading"
            class="btn btn-primary btn-lg gap-2"
            :class="{ 'loading': loading }"
          >
            <svg v-if="!loading" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
            </svg>
            Sync Schema
          </button>

          <button
            @click="fetchStatus"
            :disabled="loading"
            class="btn btn-outline btn-lg gap-2"
            :class="{ 'loading': loading }"
          >
            <svg v-if="!loading" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Refresh
          </button>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
      <div class="stats shadow">
        <div class="stat">
          <div class="stat-figure text-primary">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="inline-block w-8 h-8 stroke-current">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
            </svg>
          </div>
          <div class="stat-title">Service Status</div>
          <div class="stat-value text-primary">
            {{ syncStatus?.isRunning ? 'Active' : 'Stopped' }}
          </div>
          <div class="stat-desc">
            <span :class="syncStatus?.isRunning ? 'badge badge-success' : 'badge badge-ghost'">
              {{ syncStatus?.isRunning ? '● Running' : '○ Idle' }}
            </span>
          </div>
        </div>
      </div>

      <div class="stats shadow">
        <div class="stat">
          <div class="stat-figure text-secondary">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="inline-block w-8 h-8 stroke-current">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
          </div>
          <div class="stat-title">Schedule</div>
          <div class="stat-value text-secondary text-lg">
            {{ syncStatus?.cronSchedule || '-' }}
          </div>
          <div class="stat-desc">Cron expression</div>
        </div>
      </div>

      <div class="stats shadow">
        <div class="stat">
          <div class="stat-figure text-accent">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="inline-block w-8 h-8 stroke-current">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
          </div>
          <div class="stat-title">Last Run</div>
          <div class="stat-value text-accent text-sm">
            {{ syncStatus?.lastRun || 'Never' }}
          </div>
          <div class="stat-desc">Previous sync</div>
        </div>
      </div>

      <div class="stats shadow">
        <div class="stat">
          <div class="stat-figure text-info">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="inline-block w-8 h-8 stroke-current">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
          </div>
          <div class="stat-title">Next Run</div>
          <div class="stat-value text-info text-sm">
            {{ syncStatus?.nextRun || 'Not scheduled' }}
          </div>
          <div class="stat-desc">Upcoming sync</div>
        </div>
      </div>
    </div>

    <!-- Configuration -->
    <div class="card bg-base-100 shadow-xl mb-6">
      <div class="card-body">
        <h2 class="card-title text-xl mb-4">Configuration</h2>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <label class="label">
              <span class="label-text font-semibold">Batch Size</span>
            </label>
            <div class="badge badge-lg badge-outline">
              {{ syncStatus?.batchSize || 0 }} records/batch
            </div>
          </div>
          <div>
            <label class="label">
              <span class="label-text font-semibold">Auto Schema Sync</span>
            </label>
            <div class="badge badge-lg" :class="syncStatus?.autoSchemaSync ? 'badge-success' : 'badge-ghost'">
              {{ syncStatus?.autoSchemaSync ? 'Enabled' : 'Disabled' }}
            </div>
          </div>
          <div>
            <label class="label">
              <span class="label-text font-semibold">Total Tables</span>
            </label>
            <div class="badge badge-lg badge-primary">
              {{ tableCount }} tables
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Toast Notifications -->
    <div class="toast toast-top toast-end" v-if="notification">
      <div class="alert" :class="{
        'alert-success': notification.type === 'success',
        'alert-error': notification.type === 'error',
        'alert-info': notification.type === 'info'
      }">
        <span>{{ notification.message }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const API_BASE_URL = 'http://localhost:3000/api'

const syncStatus = ref(null)
const loading = ref(false)
const notification = ref(null)
let statusInterval = null

const tableCount = computed(() => {
  if (!syncStatus.value?.tables) return 0
  return Object.keys(syncStatus.value.tables).length
})

const showNotification = (message, type = 'info') => {
  notification.value = { message, type }
  setTimeout(() => {
    notification.value = null
  }, 3000)
}

const fetchStatus = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/sync/status`)
    const result = await response.json()

    if (result.success) {
      syncStatus.value = result.data
    }
  } catch (error) {
    console.error('Failed to fetch status:', error)
  }
}

const startSync = async () => {
  loading.value = true
  try {
    const response = await fetch(`${API_BASE_URL}/sync/start`, {
      method: 'POST'
    })
    const result = await response.json()

    if (result.success) {
      showNotification('Sync service started successfully!', 'success')
      await fetchStatus()
    } else {
      showNotification('Failed to start sync service', 'error')
    }
  } catch (error) {
    console.error('Failed to start sync:', error)
    showNotification('Error: ' + error.message, 'error')
  } finally {
    loading.value = false
  }
}

const stopSync = async () => {
  loading.value = true
  try {
    const response = await fetch(`${API_BASE_URL}/sync/stop`, {
      method: 'POST'
    })
    const result = await response.json()

    if (result.success) {
      showNotification('Sync service stopped successfully!', 'success')
      await fetchStatus()
    } else {
      showNotification('Failed to stop sync service', 'error')
    }
  } catch (error) {
    console.error('Failed to stop sync:', error)
    showNotification('Error: ' + error.message, 'error')
  } finally {
    loading.value = false
  }
}

const triggerSchemaSync = async () => {
  loading.value = true
  try {
    const response = await fetch(`${API_BASE_URL}/schema/sync`, {
      method: 'POST'
    })
    const result = await response.json()

    if (result.success) {
      showNotification('Schema sync completed successfully!', 'success')
      await fetchStatus()
    } else {
      showNotification('Schema sync failed', 'error')
    }
  } catch (error) {
    console.error('Failed to trigger schema sync:', error)
    showNotification('Error: ' + error.message, 'error')
  } finally {
    loading.value = false
  }
}

//utorefresh status setiap 5 detik
onMounted(() => {
  fetchStatus()
  statusInterval = setInterval(fetchStatus, 5000)
})

onUnmounted(() => {
  if (statusInterval) {
    clearInterval(statusInterval)
  }
})
</script>
