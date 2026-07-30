<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import MdiQrcode from "~icons/mdi/qrcode";
  import MdiLoading from "~icons/mdi/loading";
  import MdiArrowLeft from "~icons/mdi/arrow-left";
  import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
  import { Button } from "@/components/ui/button";
  import AppLogo from "~/components/App/Logo.vue";
  import { useQRLogin } from "~/composables/use-qr-login";

  const { t } = useI18n();
  const { exchange } = useQRLogin();

  useHead({
    title: "HomeBox | " + t("index.qr_login_title"),
  });

  definePageMeta({
    layout: "empty",
  });

  const route = useRoute();
  const auth = useAuthContext();

  const token = computed(() => {
    const value = route.query.token;
    return typeof value === "string" ? value : "";
  });

  const loading = ref(true);
  const failed = ref(false);

  onMounted(async () => {
    if (!token.value) {
      loading.value = false;
      failed.value = true;
      toast.error(t("index.qr_login_missing_token"));
      return;
    }

    if (auth.isAuthorized()) {
      window.location.assign("/home");
      return;
    }

    const { error } = await exchange(token.value, true);
    loading.value = false;

    if (error) {
      failed.value = true;
      toast.error(t("index.qr_login_invalid"));
      return;
    }

    toast.success(t("index.qr_login_success"));
    // Hard navigate so AuthContext picks up Set-Cookie session state.
    window.location.assign("/home");
  });
</script>

<template>
  <div class="flex min-h-screen flex-col items-center justify-center p-6">
    <div class="mb-6 flex items-center gap-2 text-3xl font-bold tracking-tight">
      HomeB
      <AppLogo class="-mb-2 w-10" />
      x
    </div>

    <Card class="md:w-[460px]">
      <CardHeader>
        <CardTitle class="flex items-center gap-2">
          <MdiQrcode class="size-6" />
          {{ $t("index.qr_login_title") }}
        </CardTitle>
      </CardHeader>

      <CardContent>
        <div v-if="loading" class="flex flex-col items-center gap-3 py-6">
          <MdiLoading class="size-8 animate-spin" />
          <p class="text-sm text-muted-foreground">{{ $t("index.qr_login_signing_in") }}</p>
        </div>
        <p v-else-if="failed" class="text-sm text-muted-foreground">{{ $t("index.qr_login_failed") }}</p>
      </CardContent>

      <CardFooter v-if="failed">
        <Button variant="secondary" as-child>
          <NuxtLink to="/">
            <MdiArrowLeft class="mr-1" />
            {{ $t("index.back_to_login") }}
          </NuxtLink>
        </Button>
      </CardFooter>
    </Card>
  </div>
</template>
