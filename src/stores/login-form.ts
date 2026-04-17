import { Ref, ref } from "vue";
import { defineStore } from "pinia";

export interface LoginForm {
  username: string;
  password: string;
  save_password: boolean | undefined;
}

export const useLoginFormStore = defineStore(
  "login_form",
  () => {
    const form: Ref<LoginForm> = ref({
      username: "",
      password: "",
      save_password: false,
    });

    return {
      form,
    };
  },
  { persist: true },
);
