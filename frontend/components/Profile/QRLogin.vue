<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import MdiQrcode from "~icons/mdi/qrcode";
  import MdiRefresh from "~icons/mdi/refresh";
  import MdiLoading from "~icons/mdi/loading";
  import { Button } from "@/components/ui/button";
  import BaseCard from "@/components/Base/Card.vue";
  import BaseSectionHeader from "@/components/Base/SectionHeader.vue";
  import { useQRLogin, useQRLoginApi } from "~/composables/use-qr-login";

  const { t } = useI18n();
  const { qrImageUrl } = useQRLogin();
  const api = useQRLoginApi(true);

  const loading = ref(false);
  const token = ref<string | null>(null);
  const secondsLeft = ref(0);
  let countdownTimer: ReturnType<typeof setInterval> | null = null;

  const imageUrl = computed(() => (token.value ? qrImageUrl(token.value) : ""));

  function clearCountdown() {
    if (countdownTimer) {
      clearInterval(countdownTimer);
      countdownTimer = null;
    }
  }

  function startCountdown(expiresAt: Date) {
    clearCountdown();
    const tick = () => {
      secondsLeft.value = Math.max(0, Math.floor((expiresAt.getTime() - Date.now()) / 1000));
      if (secondsLeft.value <= 0) clearCountdown();
    };
    tick();
    countdownTimer = setInterval(tick, 1000);
  }

  async function generate() {
    loading.value = true;
    const { data, error } = await api.create();
    loading.value = false;
    if (error || !data) {
      toast.error(t("profile.toast.failed_qr_login"));
      return;
    }
    token.value = data.token;
    startCountdown(new Date(data.expiresAt));
  }

  onBeforeUnmount(() => clearCountdown());
</script>

<template>
  <BaseCard>
    <template #title>
      <BaseSectionHeader>
        <MdiQrcode class="-mt-1 mr-2" />
        <span>{{ $t("profile.qr_login") }}</span>
        <template #description>{{ $t("profile.qr_login_sub") }}</template>
      </BaseSectionHeader>
    </template>

    <div class="px-4 pb-4">
      <p class="mb-3 text-sm text-muted-foreground">{{ $t("profile.qr_login_body") }}</p>

      <div v-if="token" class="mb-4 flex flex-col items-center gap-3">
        <div v-if="loading" class="flex h-64 w-64 items-center justify-center">
          <MdiLoading class="size-8 animate-spin" />
        </div>
        <img
          v-else-if="imageUrl && secondsLeft > 0"
          :src="imageUrl"
          :alt="$t('profile.qr_login_title')"
          class="h-64 w-64 rounded-md border bg-white p-2"
        />
        <p v-else class="text-sm text-muted-foreground">{{ $t("profile.qr_login_expired") }}</p>
        <p v-if="secondsLeft > 0" class="text-sm text-muted-foreground">
          {{ $t("profile.qr_login_expires_in", { seconds: secondsLeft }) }}
        </p>
        <p class="max-w-md text-center text-sm text-muted-foreground">
          {{ $t("profile.qr_login_description") }}
        </p>
      </div>

      <div class="flex flex-wrap gap-2">
        <Button variant="secondary" size="sm" :disabled="loading" @click="generate">
          <MdiLoading v-if="loading" class="mr-1 animate-spin" />
          <MdiQrcode v-else-if="!token" class="mr-1" />
          <MdiRefresh v-else class="mr-1" />
          {{ token ? $t("profile.qr_login_refresh") : $t("profile.qr_login_show") }}
        </Button>
      </div>
    </div>
  </BaseCard>
</template>
