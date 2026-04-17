<template>
  <div class="min-h-screen w-full flex flex-col items-center justify-center">
    <div class="flex-1 flex flex-col items-center justify-center">
      <Image :src="Logo" class="mb-4" alt="Logo" width="128" height="128" />

        <LogoutForm v-if="running" />
        <LoginForm v-else />
    </div>

    <footer>
      <GlobalFooter />
    </footer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, Ref, ref } from "vue";
// import { useToast } from "primevue/usetoast";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";

import Logo from "../assets/images/logo.png";
import { WorkerStatusPayload } from "../worker";

// const toast = useToast();
const running: Ref<WorkerStatusPayload["running"]> = ref(false); // 当前状态

async function fetchRunningStatus(): Promise<boolean> {
  const state = await invoke<WorkerStatusPayload>("get_worker_status");
  return state.running;
}

listen<WorkerStatusPayload>("worker-status", (event) => {
  const state = event.payload;
  running.value = state.running;

  // if (state.message) {
  //   toast.add({
  //     severity: "secondary",
  //     summary: "登录状态",
  //     detail: state.message,
  //     life: 5000,
  //   });
  // }
});

onMounted(async () => {
  try {
    running.value = await fetchRunningStatus();
  } catch (err) {
    console.error("获取运行状态失败:", err);
  }
});
</script>
