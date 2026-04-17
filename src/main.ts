import { createApp } from "vue";
import App from "./App.vue";

import { createPinia } from "pinia";
import PrimeVue from "primevue/config";
import Aura from "@primeuix/themes/aura";
import { definePreset } from "@primeuix/themes";
import ToastService from "primevue/toastservice";
import piniaPluginPersistedstate from "pinia-plugin-persistedstate";

import "./assets/style/index.css";
import router from "./router";
import { zhCNPrime } from "./locale";

const app = createApp(App);
const pinia = createPinia();
const skyAura = definePreset(Aura, {
  semantic: {
    primary: {
      50: "{sky.50}",
      100: "{sky.100}",
      200: "{sky.200}",
      300: "{sky.300}",
      400: "{sky.400}",
      500: "{sky.500}",
      600: "{sky.600}",
      700: "{sky.700}",
      800: "{sky.800}",
      900: "{sky.900}",
      950: "{sky.950}",
    },
  },
});

pinia.use(piniaPluginPersistedstate);

app.use(pinia);
app.use(router);
app.use(PrimeVue, {
  theme: {
    preset: skyAura,
  },
  locale: zhCNPrime,
});
app.use(ToastService);

app.mount("#app");
