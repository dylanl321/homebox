<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import MdiLoading from "~icons/mdi/loading";
  import MdiPrinter from "~icons/mdi/printer";
  import BaseCard from "@/components/Base/Card.vue";
  import BaseSectionHeader from "@/components/Base/SectionHeader.vue";
  import FormTextField from "@/components/Form/TextField.vue";
  import { Button } from "@/components/ui/button";
  import { Label } from "@/components/ui/label";
  import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
  import { toast } from "@/components/ui/sonner";
  import { route } from "~/lib/api/base";
  import type { ZebraPrinterSettings } from "~/lib/api/classes/zebra-printer";

  const { t } = useI18n();
  const api = useUserApi();
  const auth = useAuthContext();

  const loading = ref(true);
  const saving = ref(false);
  const testing = ref(false);
  const settings = ref<ZebraPrinterSettings>({
    printerIp: "10.0.1.161",
    printerPort: 9100,
    labelSize: "2x1",
    orientation: "portrait",
    darkness: 15,
    printSpeed: 4,
    printFontSize: 30,
  });

  const labelSizes: ZebraPrinterSettings["labelSize"][] = ["1x1", "2x1", "2.25x1.25", "3x2", "4x2", "4x6"];
  const sizeDimensions: Record<ZebraPrinterSettings["labelSize"], [number, number]> = {
    "1x1": [1, 1],
    "2x1": [2, 1],
    "2.25x1.25": [2.25, 1.25],
    "3x2": [3, 2],
    "4x2": [4, 2],
    "4x6": [4, 6],
  };

  const previewStyle = computed(() => {
    let [width, height] = sizeDimensions[settings.value.labelSize];
    if (settings.value.orientation === "landscape") {
      [width, height] = [height, width];
    }
    return {
      aspectRatio: `${width} / ${height}`,
      width: width >= height ? "min(100%, 28rem)" : "min(100%, 15rem)",
    };
  });

  const qrPreviewURL = computed(() => {
    const base = import.meta.client ? window.location.origin : "";
    return route("/qrcode", { data: encodeURIComponent(`${base}/profile`) });
  });

  const previewTitleFont = computed(() => {
    return `${Math.max(11, Math.min(24, settings.value.printFontSize * 0.55))}px`;
  });

  const previewBodyFont = computed(() => {
    return `${Math.max(9, Math.min(16, settings.value.printFontSize * 0.38))}px`;
  });

  async function load() {
    loading.value = true;
    const { data, error } = await api.zebraPrinter.settings();
    if (error || !data) {
      toast.error(t("profile.zebra_printer_load_failed"));
    } else {
      settings.value = data;
    }
    loading.value = false;
  }

  async function save() {
    saving.value = true;
    const { data, error } = await api.zebraPrinter.update(settings.value);
    if (error || !data) {
      toast.error(t("profile.zebra_printer_save_failed"));
    } else {
      settings.value = data;
      toast.success(t("profile.zebra_printer_saved"));
    }
    saving.value = false;
  }

  async function testPrint() {
    testing.value = true;
    const { error } = await api.zebraPrinter.test(settings.value);
    if (error) {
      toast.error(t("profile.zebra_printer_test_failed"));
    } else {
      toast.success(t("profile.zebra_printer_test_sent"));
    }
    testing.value = false;
  }

  onMounted(load);
</script>

<template>
  <BaseCard>
    <template #title>
      <BaseSectionHeader>
        <MdiPrinter class="mr-2" />
        <span>{{ $t("profile.zebra_printer") }}</span>
        <template #description>{{ $t("profile.zebra_printer_sub") }}</template>
      </BaseSectionHeader>
    </template>

    <div v-if="loading" class="p-4 text-sm text-muted-foreground">
      {{ $t("global.loading") }}
    </div>

    <div v-else class="grid gap-6 p-4 lg:grid-cols-[minmax(0,1fr)_minmax(18rem,28rem)]">
      <div class="grid gap-4 sm:grid-cols-2">
        <FormTextField v-model="settings.printerIp" :label="$t('profile.zebra_printer_ip')" autocomplete="off" />
        <FormTextField
          v-model="settings.printerPort"
          type="number"
          :min="1"
          :max="65535"
          :label="$t('profile.zebra_printer_port')"
        />

        <div class="flex flex-col gap-1.5">
          <Label>{{ $t("profile.zebra_label_size") }}</Label>
          <Select
            :model-value="settings.labelSize"
            @update:model-value="value => (settings.labelSize = value as ZebraPrinterSettings['labelSize'])"
          >
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="size in labelSizes" :key="size" :value="size">{{ size }} in</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="flex flex-col gap-1.5">
          <Label>{{ $t("profile.zebra_orientation") }}</Label>
          <Select
            :model-value="settings.orientation"
            @update:model-value="value => (settings.orientation = value as ZebraPrinterSettings['orientation'])"
          >
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="portrait">{{ $t("profile.zebra_portrait") }}</SelectItem>
              <SelectItem value="landscape">{{ $t("profile.zebra_landscape") }}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <FormTextField
          v-model="settings.darkness"
          type="number"
          :min="0"
          :max="30"
          :label="$t('profile.zebra_darkness')"
        />
        <FormTextField
          v-model="settings.printSpeed"
          type="number"
          :min="1"
          :max="14"
          :label="$t('profile.zebra_print_speed')"
        />
        <FormTextField
          v-model="settings.printFontSize"
          type="number"
          :min="20"
          :max="56"
          :label="$t('profile.zebra_font_size')"
        />

        <div class="flex items-end gap-2">
          <Button :disabled="saving || testing" @click="save">
            <MdiLoading v-if="saving" class="mr-2 animate-spin" />
            {{ $t("global.save") }}
          </Button>
          <Button variant="secondary" :disabled="saving || testing" @click="testPrint">
            <MdiLoading v-if="testing" class="mr-2 animate-spin" />
            {{ $t("profile.zebra_test_print") }}
          </Button>
        </div>
      </div>

      <div>
        <Label>{{ $t("profile.zebra_preview") }}</Label>
        <div
          class="mt-1.5 flex items-stretch overflow-hidden border-2 border-foreground bg-white p-2 text-black"
          :style="previewStyle"
        >
          <div class="flex h-full w-[44%] shrink-0 items-center justify-center pr-2">
            <img :src="qrPreviewURL" class="max-h-full max-w-full object-contain" style="aspect-ratio: 1 / 1" alt="" />
          </div>
          <div
            class="flex min-w-0 flex-1 flex-col justify-center overflow-hidden border-l border-black/20 pl-2 text-left"
          >
            <strong
              class="line-clamp-2 break-words leading-tight"
              :style="{ fontSize: previewTitleFont, overflowWrap: 'anywhere' }"
            >
              {{ $t("profile.zebra_test_label_title") }}
            </strong>
            <span
              class="mt-1 line-clamp-3 break-words leading-tight"
              :style="{ fontSize: previewBodyFont, overflowWrap: 'anywhere' }"
            >
              {{ auth.user?.name || "Homebox" }}<br />
              {{ settings.printerIp }}:{{ settings.printerPort }}
            </span>
          </div>
        </div>
        <p class="mt-2 text-xs text-muted-foreground">
          {{ $t("profile.zebra_preview_note") }}
        </p>
      </div>
    </div>
  </BaseCard>
</template>
