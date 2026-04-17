<template>
  <div class="mb-6 text-center">
    <h1 class="text-4xl">{{ loginFormStore.form.username }}</h1>
    <Tag
    class="mt-2"
      :severity="
        ['Starting', 'LoggingIn', 'Paused', 'LoggingOut'].includes(status)
          ? 'warn'
          : ['NotLoggedIn', 'Stopped'].includes(status)
            ? 'danger'
            : status === 'LoggedIn'
              ? 'success'
              : 'primary'
      "
      :value="statusText"
    ></Tag>
  </div>

  <Form
    v-slot="$form"
    @submit="onLogout"
    class="flex flex-col gap-4 w-full sm:w-56"
  >
    <div class="flex flex-col gap-1">
      <Button
        type="submit"
        severity="danger"
        :label="loading ? '登出中…' : '登出'"
        :disabled="loading || !$form.valid"
      />
    </div>
  </Form>
</template>

<script setup lang="ts">
import { computed, Ref, ref } from "vue";
import { useToast } from "primevue/usetoast";
import { invoke } from "@tauri-apps/api/core";
import { FormSubmitEvent } from "@primevue/forms";

import { WorkerStatusPayload } from "../worker";
import { listen } from "@tauri-apps/api/event";
import { useLoginFormStore } from "../stores/login-form";

const toast = useToast();
const loading = ref(false); // 登出中
const loginFormStore = useLoginFormStore();
const status: Ref<WorkerStatusPayload["status"]> = ref("Starting"); // 当前状态
const statusTextMap: Record<WorkerStatusPayload["status"], string> = {
  // 当前状态文本映射
  Starting: "启动中",
  NotLoggedIn: "登录失败",
  LoggingIn: "登录中",
  LoggedIn: "已登录",
  Paused: "已暂停",
  LoggingOut: "登出中",
  Stopped: "已停止",
};
const statusText = computed(() => statusTextMap[status.value]); // 当前状态文本

/**
 * 登出逻辑
 * @param event 事件
 */
const onLogout = async (event: FormSubmitEvent) => {
  if (!event.valid || loading.value) return;
  try {
    loading.value = true;
    const state = await invoke<WorkerStatusPayload>("stop_worker");
    if (state.running) {
      toast.add({
        severity: "warning",
        summary: "登出中…",
        ...(state.message ? { detail: state.message } : {}),
        life: 5000,
      });
    } else {
      toast.add({
        severity: "success",
        summary: "登出成功",
        life: 5000,
      });
    }
  } catch (err) {
    toast.add({
      severity: "error",
      summary: "登出失败",
      detail: String(err),
      life: 5000,
    });
  } finally {
    loading.value = false;
  }
};

// 监听当前状态
listen<WorkerStatusPayload>("worker-status", (event) => {
  const state = event.payload;
  status.value = state.status;
});
</script>
