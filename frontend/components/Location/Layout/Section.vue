<script setup lang="ts">
  import { useMediaQuery, useLocalStorage } from "@vueuse/core";
  import { useI18n } from "vue-i18n";
  import { List, Map, Pencil, Scan, Trash2, ZoomIn, ZoomOut } from "lucide-vue-next";
  import { toast } from "@/components/ui/sonner";
  import type {
    EntityOut,
    EntitySummary,
    LocationLayoutOut,
    LocationLayoutReplace,
  } from "~~/lib/api/types/data-contracts";
  import LayoutCanvas from "./Canvas.vue";
  import LayoutEditor from "./Editor.vue";
  import { elementsFromLayout, isRevisionConflict } from "./layout";
  import LocationCard from "~/components/Location/Card.vue";
  import BaseSectionHeader from "@/components/Base/SectionHeader.vue";
  import { Button, ButtonGroup } from "@/components/ui/button";

  const props = defineProps<{
    locationId: string;
    children: (EntityOut | EntitySummary)[];
  }>();

  const api = useUserApi();
  const { t } = useI18n();
  const confirm = useConfirm();
  const isDesktop = useMediaQuery("(min-width: 768px)");
  const viewStorageKey = `homebox/location-layout-view/${props.locationId}`;
  const hasStoredView = import.meta.client && localStorage.getItem(viewStorageKey) !== null;
  const view = useLocalStorage<"list" | "overhead">(viewStorageKey, "list");
  const layout = ref<LocationLayoutOut>({
    canvasWidth: 1000,
    canvasHeight: 700,
    revision: 0,
    walls: [],
    locations: [],
  });
  const loading = ref(true);
  const saving = ref(false);
  const editing = ref(false);
  const canvas = ref<InstanceType<typeof LayoutCanvas>>();

  const elements = computed(() => elementsFromLayout(layout.value));
  const placedIds = computed(() => new Set(layout.value.locations.map(location => location.targetId)));
  const unplaced = computed(() => props.children.filter(child => !placedIds.value.has(child.id)));

  async function load() {
    loading.value = true;
    const response = await api.items.getLocationLayout(props.locationId);
    loading.value = false;
    if (response.error) {
      toast.error(t("locations.layout.failed_load"));
      return;
    }
    layout.value = response.data;
    if (!hasStoredView && layout.value.revision > 0) view.value = "overhead";
    else if (layout.value.revision === 0 && view.value === "overhead") view.value = "list";
  }

  async function save(value: LocationLayoutReplace) {
    saving.value = true;
    const response = await api.items.replaceLocationLayout(props.locationId, value);
    saving.value = false;
    if (isRevisionConflict(response.status)) {
      toast.error(t("locations.layout.conflict"));
      editing.value = false;
      await load();
      return;
    }
    if (response.error) {
      toast.error(t("locations.layout.failed_save"));
      return;
    }
    layout.value = response.data;
    editing.value = false;
    view.value = "overhead";
    toast.success(t("locations.layout.saved"));
  }

  async function deleteLayout() {
    const result = await confirm.open(t("locations.layout.delete_confirm"));
    if (result.isCanceled) return;
    const response = await api.items.deleteLocationLayout(props.locationId);
    if (response.error) {
      toast.error(t("locations.layout.failed_delete"));
      return;
    }
    layout.value = {
      canvasWidth: 1000,
      canvasHeight: 700,
      revision: 0,
      walls: [],
      locations: [],
    };
    editing.value = false;
    view.value = "list";
    toast.success(t("locations.layout.deleted"));
  }

  function activate(targetId: string) {
    navigateTo(`/location/${targetId}`);
  }

  onMounted(load);
</script>

<template>
  <section class="mt-6" data-testid="location-layout-section">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
      <BaseSectionHeader class="!pb-0">{{ $t("locations.child_locations") }}</BaseSectionHeader>
      <ButtonGroup>
        <Button
          size="sm"
          :variant="view === 'list' ? 'default' : 'outline'"
          :title="$t('locations.layout.list')"
          @click="view = 'list'"
        >
          <List class="size-4" />
          {{ $t("locations.layout.list") }}
        </Button>
        <Button
          size="sm"
          :variant="view === 'overhead' ? 'default' : 'outline'"
          :title="$t('locations.layout.overhead')"
          @click="view = 'overhead'"
        >
          <Map class="size-4" />
          {{ $t("locations.layout.overhead") }}
        </Button>
      </ButtonGroup>
    </div>

    <div v-if="loading" class="h-40 animate-pulse rounded-md border bg-muted/40" />

    <div v-else-if="view === 'list'" class="grid grid-cols-1 gap-2 sm:grid-cols-3">
      <LocationCard v-for="child in children" :key="child.id" :location="child" />
      <p v-if="children.length === 0" class="text-sm text-muted-foreground">
        {{ $t("locations.layout.no_children") }}
      </p>
    </div>

    <LayoutEditor
      v-else-if="editing && isDesktop"
      :layout="layout"
      :children="children"
      :saving="saving"
      @save="save"
      @cancel="editing = false"
    />

    <div v-else class="space-y-3">
      <div class="flex flex-wrap justify-end gap-2">
        <ButtonGroup>
          <Button size="icon" variant="outline" :title="$t('locations.layout.zoom_out')" @click="canvas?.zoomOut()">
            <ZoomOut class="size-4" />
          </Button>
          <Button size="icon" variant="outline" :title="$t('locations.layout.fit')" @click="canvas?.fit()">
            <Scan class="size-4" />
          </Button>
          <Button size="icon" variant="outline" :title="$t('locations.layout.zoom_in')" @click="canvas?.zoomIn()">
            <ZoomIn class="size-4" />
          </Button>
        </ButtonGroup>
        <Button v-if="isDesktop" size="sm" @click="editing = true">
          <Pencil class="size-4" />
          {{ layout.revision ? $t("locations.layout.edit") : $t("locations.layout.create") }}
        </Button>
        <Button
          v-if="isDesktop && layout.revision"
          size="icon"
          variant="destructive"
          :title="$t('global.delete')"
          @click="deleteLayout"
        >
          <Trash2 class="size-4" />
        </Button>
      </div>

      <LayoutCanvas ref="canvas" :elements="elements" :children="children" @activate="activate" />

      <div v-if="unplaced.length > 0">
        <h3 class="mb-2 text-sm font-semibold">
          {{ $t("locations.layout.unplaced") }}
        </h3>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <LocationCard v-for="child in unplaced" :key="child.id" :location="child" dense />
        </div>
      </div>
    </div>
  </section>
</template>
