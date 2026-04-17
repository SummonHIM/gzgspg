<template>
  <div class="mb-6 text-center">
    <h3>广州工商学院</h3>
    <h1 class="text-4xl">校园网登陆器</h1>
  </div>

  <Form
    v-slot="$form"
    :initial-values="loginFormStore.form"
    :resolver="loginFormResolver"
    @submit="onLogin"
    class="flex flex-col gap-4 w-full sm:w-56"
  >
    <div class="flex flex-col gap-1">
      <FloatLabel variant="on">
        <InputText id="login-form-username" name="username" type="text" fluid />
        <label for="login-form-username">用户名</label>
      </FloatLabel>
      <Message
        v-if="$form.username?.invalid"
        severity="error"
        size="small"
        variant="simple"
      >
        {{ $form.username.error?.message }}
      </Message>

      <FloatLabel variant="on">
        <Password
          id="login-form-password"
          name="password"
          type="text"
          :feedback="false"
          fluid
        />
        <label for="login-form-password">密码</label>
      </FloatLabel>
      <Message
        v-if="$form.password?.invalid"
        severity="error"
        size="small"
        variant="simple"
      >
        {{ $form.password.error?.message }}
      </Message>

      <div class="flex flex-row gap-2 mt-2">
        <ToggleSwitch name="save_password" />
        <p>保存密码</p>
      </div>
    </div>

    <Button
      type="submit"
      severity="primary"
      :label="loading ? '登陆中…' : '登录'"
      :disabled="loading || !$form.valid"
    />
  </Form>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useToast } from "primevue/usetoast";
import { invoke } from "@tauri-apps/api/core";
import { FormResolverOptions, FormSubmitEvent } from "@primevue/forms";

import { StartWorkerResponse } from "../worker";
import { LoginForm, useLoginFormStore } from "../stores/login-form";

const toast = useToast();
const loginFormStore = useLoginFormStore();
const loading = ref(false);

/**
 * 登录表单验证器
 * @param param0 表单内容
 */
const loginFormResolver = ({ values }: FormResolverOptions) => {
  const errors: Partial<Record<keyof LoginForm, { message: string }[]>> = {};
  if (!values.username) {
    errors.username = [{ message: "请输入用户名。" }];
  }
  if (!values.password) {
    errors.password = [{ message: "请输入密码。" }];
  }
  return {
    values,
    errors,
  };
};

const onLogin = async (event: FormSubmitEvent) => {
  if (!event.valid || loading.value) return;
  console.log(event);

  const { username, password, save_password } = event.values;
  loginFormStore.form.username = username;
  loginFormStore.form.save_password = save_password;

  if (save_password === true) loginFormStore.form.password = password;
  else loginFormStore.form.password = "";

  try {
    loading.value = true;
    const resp = await invoke<StartWorkerResponse>("start_worker", {
      username,
      password,
    });
    if (resp.alreadyRunning) {
      toast.add({
        severity: "warn",
        summary: "守护进程已在运行",
        life: 5000,
      });
    }
  } catch (err) {
    toast.add({
      severity: "error",
      summary: "登录失败",
      detail: String(err),
      life: 5000,
    });
  } finally {
    loading.value = false;
  }
};
</script>
