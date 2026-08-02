<script setup lang="ts">
  import {
    Check,
    Grid3X3,
    Hand,
    Link,
    MapPin,
    MousePointer2,
    Redo2,
    Save,
    Trash2,
    Undo2,
    Unlink,
    BrickWall,
    X,
    ZoomIn,
    ZoomOut,
    Scan,
  } from "lucide-vue-next";
  import type {
    LocationLayoutElementInput,
    LocationLayoutOut,
    LocationLayoutReplace,
  } from "~~/lib/api/types/data-contracts";
  import type { ChildLocation, LayoutTool } from "./layout";
  import {
    cloneElements,
    commitHistory,
    elementsForSave,
    elementsFromLayout,
    redoHistory,
    undoHistory,
    unplacedChildren,
  } from "./layout";
  import LayoutCanvas from "./Canvas.vue";
  import { Button, ButtonGroup } from "@/components/ui/button";

  const props = defineProps<{
    layout: LocationLayoutOut;
    children: ChildLocation[];
    saving?: boolean;
  }>();

  const emit = defineEmits<{
    save: [value: LocationLayoutReplace];
    cancel: [];
  }>();

  const elements = ref<LocationLayoutElementInput[]>(elementsFromLayout(props.layout));
  const selectedId = ref("");
  const tool = ref<LayoutTool>("select");
  const gridSnap = ref(true);
  const endpointSnap = ref(true);
  const past = ref<LocationLayoutElementInput[][]>([]);
  const future = ref<LocationLayoutElementInput[][]>([]);
  const interactionSnapshot = ref<LocationLayoutElementInput[]>();
  const canvas = ref<InstanceType<typeof LayoutCanvas>>();

  const unplaced = computed(() => unplacedChildren(props.children, elements.value));
  const canUndo = computed(() => past.value.length > 0);
  const canRedo = computed(() => future.value.length > 0);

  function interactionStart() {
    interactionSnapshot.value = cloneElements(elements.value);
  }

  function interactionEnd() {
    const before = interactionSnapshot.value;
    interactionSnapshot.value = undefined;
    if (!before) return;
    commitHistory(past.value, future.value, before, elements.value);
  }

  function undo() {
    elements.value = undoHistory(past.value, future.value, elements.value);
    selectedId.value = "";
  }

  function redo() {
    elements.value = redoHistory(past.value, future.value, elements.value);
    selectedId.value = "";
  }

  function removeSelected() {
    if (!selectedId.value) return;
    past.value.push(cloneElements(elements.value));
    elements.value = elements.value.filter(element => element.id !== selectedId.value);
    selectedId.value = "";
    future.value = [];
  }

  function save() {
    emit("save", {
      expectedRevision: props.layout.revision,
      elements: elementsForSave(elements.value),
    });
  }

  function dragChild(event: DragEvent, id: string) {
    event.dataTransfer?.setData("application/x-homebox-location", id);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = "copy";
  }

  function onKeydown(event: KeyboardEvent) {
    const target = event.target as HTMLElement;
    if (["INPUT", "TEXTAREA"].includes(target.tagName)) return;
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "z") {
      event.preventDefault();
      if (event.shiftKey) redo();
      else undo();
    } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "y") {
      event.preventDefault();
      redo();
    } else if (event.key === "Delete" || event.key === "Backspace") {
      event.preventDefault();
      removeSelected();
    }
  }

  onMounted(() => window.addEventListener("keydown", onKeydown));
  onBeforeUnmount(() => window.removeEventListener("keydown", onKeydown));
</script>

<template>
  <div class="space-y-3" data-testid="location-layout-editor">
    <div class="flex flex-wrap items-center gap-2 border-y py-2">
      <ButtonGroup>
        <Button
          size="icon"
          :variant="tool === 'select' ? 'default' : 'outline'"
          :title="$t('locations.layout.select')"
          @click="tool = 'select'"
        >
          <MousePointer2 class="size-4" />
        </Button>
        <Button
          size="icon"
          :variant="tool === 'pan' ? 'default' : 'outline'"
          :title="$t('locations.layout.pan')"
          @click="tool = 'pan'"
        >
          <Hand class="size-4" />
        </Button>
        <Button
          size="icon"
          :variant="tool === 'wall' ? 'default' : 'outline'"
          :title="$t('locations.layout.wall')"
          @click="tool = 'wall'"
        >
          <BrickWall class="size-4" />
        </Button>
      </ButtonGroup>

      <ButtonGroup>
        <Button size="icon" variant="outline" :disabled="!canUndo" :title="$t('locations.layout.undo')" @click="undo">
          <Undo2 class="size-4" />
        </Button>
        <Button size="icon" variant="outline" :disabled="!canRedo" :title="$t('locations.layout.redo')" @click="redo">
          <Redo2 class="size-4" />
        </Button>
        <Button
          size="icon"
          variant="outline"
          :disabled="!selectedId"
          :title="$t('global.delete')"
          @click="removeSelected"
        >
          <Trash2 class="size-4" />
        </Button>
      </ButtonGroup>

      <Button
        size="sm"
        :variant="gridSnap ? 'secondary' : 'outline'"
        :title="$t('locations.layout.grid_snap')"
        @click="gridSnap = !gridSnap"
      >
        <Grid3X3 class="size-4" />
        {{ $t("locations.layout.grid") }}
        <Check v-if="gridSnap" class="size-3" />
      </Button>
      <Button
        size="sm"
        :variant="endpointSnap ? 'secondary' : 'outline'"
        :title="$t('locations.layout.endpoint_snap')"
        @click="endpointSnap = !endpointSnap"
      >
        <Link v-if="endpointSnap" class="size-4" />
        <Unlink v-else class="size-4" />
        {{ $t("locations.layout.endpoints") }}
      </Button>

      <ButtonGroup class="ml-auto">
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
    </div>

    <div class="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_220px]">
      <LayoutCanvas
        ref="canvas"
        v-model:elements="elements"
        v-model:selected-id="selectedId"
        :children="children"
        :tool="tool"
        :grid-snap="gridSnap"
        :endpoint-snap="endpointSnap"
        editable
        @interaction-start="interactionStart"
        @interaction-end="interactionEnd"
      />

      <aside class="min-w-0 border-l pl-4">
        <h3 class="mb-2 text-sm font-semibold">
          {{ $t("locations.layout.unplaced") }}
        </h3>
        <div class="space-y-2">
          <button
            v-for="child in unplaced"
            :key="child.id"
            draggable="true"
            class="flex w-full cursor-grab items-center gap-2 rounded-md border bg-card px-3 py-2 text-left text-sm hover:bg-accent active:cursor-grabbing"
            @dragstart="dragChild($event, child.id)"
          >
            <MapPin class="size-4 shrink-0 text-muted-foreground" />
            <span class="min-w-0 truncate">{{ child.name }}</span>
          </button>
          <p v-if="unplaced.length === 0" class="text-sm text-muted-foreground">
            {{ $t("locations.layout.all_placed") }}
          </p>
        </div>
      </aside>
    </div>

    <div class="flex justify-end gap-2 border-t pt-3">
      <Button variant="outline" :disabled="saving" @click="emit('cancel')">
        <X class="size-4" />
        {{ $t("global.cancel") }}
      </Button>
      <Button :disabled="saving" @click="save">
        <Save class="size-4" />
        {{ saving ? $t("global.loading") : $t("global.save") }}
      </Button>
    </div>
  </div>
</template>
