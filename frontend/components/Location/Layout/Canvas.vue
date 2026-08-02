<script setup lang="ts">
  import type { LocationLayoutElementInput } from "~~/lib/api/types/data-contracts";
  import type { ChildLocation, LayoutTool, Point } from "./layout";
  import {
    clamp,
    cloneElements,
    createPlacement,
    emptyElement,
    locationItemCount,
    locationName,
    normalizeRotation,
    snapPoint,
  } from "./layout";

  const props = withDefaults(
    defineProps<{
      elements: LocationLayoutElementInput[];
      children: ChildLocation[];
      editable?: boolean;
      tool?: LayoutTool;
      gridSnap?: boolean;
      endpointSnap?: boolean;
      selectedId?: string;
    }>(),
    {
      editable: false,
      tool: "select",
      gridSnap: true,
      endpointSnap: true,
      selectedId: "",
    }
  );

  const emit = defineEmits<{
    "update:elements": [value: LocationLayoutElementInput[]];
    "update:selectedId": [value: string];
    "interaction-start": [];
    "interaction-end": [];
    activate: [targetId: string];
  }>();

  const canvasWidth = 1000;
  const canvasHeight = 700;
  const wrapper = ref<HTMLElement>();
  const svg = ref<SVGSVGElement>();
  const zoom = ref(1);
  const pan = reactive({ x: 0, y: 0 });

  type Interaction =
    | { kind: "pan"; start: Point; pan: Point; moved: boolean }
    | { kind: "wall"; id: string }
    | {
        kind: "move";
        id: string;
        start: Point;
        original: LocationLayoutElementInput;
      }
    | {
        kind: "resize";
        id: string;
        start: Point;
        original: LocationLayoutElementInput;
      }
    | {
        kind: "rotate";
        id: string;
        center: Point;
        initialAngle: number;
        originalRotation: number;
      }
    | { kind: "wall-start" | "wall-end"; id: string };

  const interaction = ref<Interaction>();

  const childMap = computed(() => new Map(props.children.map(child => [child.id, child])));
  const sortedElements = computed(() => [...props.elements].sort((a, b) => a.zOrder - b.zOrder));
  const selected = computed(() => props.elements.find(element => element.id === props.selectedId));

  function displayName(element: LocationLayoutElementInput) {
    const child = childMap.value.get(element.targetId);
    return child ? locationName(child) : "";
  }

  function displayCount(element: LocationLayoutElementInput) {
    const child = childMap.value.get(element.targetId);
    return child ? locationItemCount(child) : 0;
  }

  function toCanvas(event: PointerEvent | DragEvent): Point {
    const rect = svg.value!.getBoundingClientRect();
    const rawX = ((event.clientX - rect.left) / rect.width) * canvasWidth;
    const rawY = ((event.clientY - rect.top) / rect.height) * canvasHeight;
    return {
      x: clamp((rawX - pan.x) / zoom.value / canvasWidth),
      y: clamp((rawY - pan.y) / zoom.value / canvasHeight),
    };
  }

  function updateElement(id: string, patch: Partial<LocationLayoutElementInput>) {
    emit(
      "update:elements",
      props.elements.map(element => (element.id === id ? { ...element, ...patch } : element))
    );
  }

  function startBackground(event: PointerEvent) {
    if (event.button !== 0) return;
    if (!(props.editable && props.tool === "wall") && event.target !== svg.value) return;
    svg.value?.setPointerCapture(event.pointerId);
    const point = toCanvas(event);

    if (props.editable && props.tool === "wall") {
      emit("interaction-start");
      const snapped = snapPoint(point, props.elements, props.gridSnap, props.endpointSnap);
      const wall = {
        ...emptyElement("wall"),
        x: snapped.x,
        y: snapped.y,
        endX: snapped.x,
        endY: snapped.y,
        zOrder: props.elements.length,
      };
      emit("update:elements", [...cloneElements(props.elements), wall]);
      emit("update:selectedId", wall.id);
      interaction.value = { kind: "wall", id: wall.id };
      return;
    }

    if (!props.editable || props.tool === "pan") {
      interaction.value = {
        kind: "pan",
        start: { x: event.clientX, y: event.clientY },
        pan: { ...pan },
        moved: false,
      };
      return;
    }
    emit("update:selectedId", "");
  }

  function startElement(event: PointerEvent, element: LocationLayoutElementInput) {
    if (!props.editable || props.tool !== "select") return;
    event.stopPropagation();
    emit("update:selectedId", element.id);
    svg.value?.setPointerCapture(event.pointerId);
    emit("interaction-start");
    interaction.value = {
      kind: "move",
      id: element.id,
      start: toCanvas(event),
      original: { ...element },
    };
  }

  function startHandle(event: PointerEvent, kind: "resize" | "rotate" | "wall-start" | "wall-end") {
    const element = selected.value;
    if (!element) return;
    event.stopPropagation();
    svg.value?.setPointerCapture(event.pointerId);
    emit("interaction-start");
    const point = toCanvas(event);
    if (kind === "resize") {
      interaction.value = {
        kind,
        id: element.id,
        start: point,
        original: { ...element },
      };
    } else if (kind === "rotate") {
      const center = {
        x: element.x + element.width / 2,
        y: element.y + element.height / 2,
      };
      interaction.value = {
        kind,
        id: element.id,
        center,
        initialAngle: Math.atan2(point.y - center.y, point.x - center.x),
        originalRotation: element.rotation,
      };
    } else {
      interaction.value = { kind, id: element.id };
    }
  }

  function onPointerMove(event: PointerEvent) {
    const active = interaction.value;
    if (!active) return;

    if (active.kind === "pan") {
      const dx = event.clientX - active.start.x;
      const dy = event.clientY - active.start.y;
      active.moved ||= Math.abs(dx) + Math.abs(dy) > 4;
      const rect = svg.value!.getBoundingClientRect();
      pan.x = active.pan.x + (dx / rect.width) * canvasWidth;
      pan.y = active.pan.y + (dy / rect.height) * canvasHeight;
      return;
    }

    const point = toCanvas(event);
    if (active.kind === "wall") {
      const snapped = snapPoint(point, props.elements, props.gridSnap, props.endpointSnap, active.id);
      updateElement(active.id, { endX: snapped.x, endY: snapped.y });
    } else if (active.kind === "move") {
      const dx = point.x - active.start.x;
      const dy = point.y - active.start.y;
      if (active.original.kind === "wall") {
        const width = active.original.endX - active.original.x;
        const height = active.original.endY - active.original.y;
        const x = clamp(active.original.x + dx, Math.min(0, -width), Math.min(1, 1 - width));
        const y = clamp(active.original.y + dy, Math.min(0, -height), Math.min(1, 1 - height));
        updateElement(active.id, { x, y, endX: x + width, endY: y + height });
      } else {
        const snapped = snapPoint(
          {
            x: clamp(active.original.x + dx, 0, 1 - active.original.width),
            y: clamp(active.original.y + dy, 0, 1 - active.original.height),
          },
          props.elements,
          props.gridSnap,
          false,
          active.id
        );
        updateElement(active.id, {
          x: clamp(snapped.x, 0, 1 - active.original.width),
          y: clamp(snapped.y, 0, 1 - active.original.height),
        });
      }
    } else if (active.kind === "resize") {
      const width = clamp(active.original.width + point.x - active.start.x, 0.04, 1 - active.original.x);
      const height = clamp(active.original.height + point.y - active.start.y, 0.04, 1 - active.original.y);
      updateElement(active.id, { width, height });
    } else if (active.kind === "rotate") {
      const angle = Math.atan2(point.y - active.center.y, point.x - active.center.x);
      const degrees = ((angle - active.initialAngle) * 180) / Math.PI;
      updateElement(active.id, {
        rotation: normalizeRotation(active.originalRotation + Math.round(degrees / 5) * 5),
      });
    } else {
      const snapped = snapPoint(point, props.elements, props.gridSnap, props.endpointSnap, active.id);
      updateElement(
        active.id,
        active.kind === "wall-start" ? { x: snapped.x, y: snapped.y } : { endX: snapped.x, endY: snapped.y }
      );
    }
  }

  function finishInteraction(event: PointerEvent) {
    if (!interaction.value) return;
    const wasEdit = interaction.value.kind !== "pan";
    if (interaction.value.kind === "wall") {
      const wallId = interaction.value.id;
      const wall = props.elements.find(element => element.id === wallId);
      if (wall && Math.hypot(wall.endX - wall.x, wall.endY - wall.y) < 0.005) {
        emit(
          "update:elements",
          props.elements.filter(element => element.id !== wall.id)
        );
      }
    }
    interaction.value = undefined;
    if (wasEdit) emit("interaction-end");
    if (svg.value?.hasPointerCapture(event.pointerId)) svg.value.releasePointerCapture(event.pointerId);
  }

  function onWheel(event: WheelEvent) {
    event.preventDefault();
    const next = clamp(zoom.value * (event.deltaY > 0 ? 0.9 : 1.1), 0.6, 3);
    zoom.value = next;
  }

  function zoomIn() {
    zoom.value = clamp(zoom.value * 1.2, 0.6, 3);
  }
  function zoomOut() {
    zoom.value = clamp(zoom.value / 1.2, 0.6, 3);
  }
  function fit() {
    zoom.value = 1;
    pan.x = 0;
    pan.y = 0;
  }

  function onDrop(event: DragEvent) {
    if (!props.editable) return;
    const targetId = event.dataTransfer?.getData("application/x-homebox-location");
    if (!targetId || props.elements.some(element => element.targetId === targetId)) return;
    const point = toCanvas(event);
    const element = createPlacement(targetId, point, props.elements.length);
    emit("interaction-start");
    emit("update:elements", [...cloneElements(props.elements), element]);
    emit("update:selectedId", element.id);
    emit("interaction-end");
  }

  function activate(event: MouseEvent | KeyboardEvent, element: LocationLayoutElementInput) {
    if (props.editable) return;
    event.stopPropagation();
    emit("activate", element.targetId);
  }

  defineExpose({ zoomIn, zoomOut, fit });
</script>

<template>
  <div
    ref="wrapper"
    data-testid="location-layout-canvas"
    class="relative aspect-[10/7] w-full overflow-hidden rounded-md border bg-background"
    :class="{
      'cursor-crosshair': editable && tool === 'wall',
      'cursor-grab': !editable || tool === 'pan',
    }"
    @dragover.prevent
    @drop.prevent="onDrop"
  >
    <svg
      ref="svg"
      class="size-full touch-none select-none"
      :viewBox="`0 0 ${canvasWidth} ${canvasHeight}`"
      role="img"
      aria-label="Overhead location layout"
      @pointerdown="startBackground"
      @pointermove="onPointerMove"
      @pointerup="finishInteraction"
      @pointercancel="finishInteraction"
      @wheel="onWheel"
    >
      <defs>
        <pattern id="location-layout-grid" width="25" height="25" patternUnits="userSpaceOnUse">
          <path d="M 25 0 L 0 0 0 25" fill="none" class="stroke-border" stroke-width="1" />
        </pattern>
        <template v-for="element in sortedElements" :key="`clip-${element.id}`">
          <clipPath v-if="element.kind === 'location'" :id="`location-layout-clip-${element.id}`">
            <rect
              :x="element.x * canvasWidth + 8"
              :y="element.y * canvasHeight + 8"
              :width="Math.max(0, element.width * canvasWidth - 16)"
              :height="Math.max(0, element.height * canvasHeight - 16)"
              rx="3"
            />
          </clipPath>
        </template>
      </defs>
      <rect width="100%" height="100%" class="fill-muted/20" pointer-events="none" />
      <g :transform="`translate(${pan.x} ${pan.y}) scale(${zoom})`">
        <rect
          v-if="editable && gridSnap"
          width="1000"
          height="700"
          fill="url(#location-layout-grid)"
          pointer-events="none"
        />

        <template v-for="element in sortedElements" :key="element.id">
          <line
            v-if="element.kind === 'wall'"
            data-layout-kind="wall"
            :data-element-id="element.id"
            :x1="element.x * canvasWidth"
            :y1="element.y * canvasHeight"
            :x2="element.endX * canvasWidth"
            :y2="element.endY * canvasHeight"
            class="stroke-foreground"
            :class="{ '!stroke-primary': element.id === selectedId }"
            stroke-width="8"
            stroke-linecap="round"
            @pointerdown="startElement($event, element)"
          />
          <g
            v-else
            data-layout-kind="location"
            :data-element-id="element.id"
            :data-target-id="element.targetId"
            :transform="`rotate(${element.rotation} ${(element.x + element.width / 2) * canvasWidth} ${(element.y + element.height / 2) * canvasHeight})`"
            class="outline-none"
            :class="{
              'cursor-pointer': !editable,
              'cursor-move': editable && tool === 'select',
            }"
            :tabindex="editable ? -1 : 0"
            role="link"
            @pointerdown="startElement($event, element)"
            @click="activate($event, element)"
            @keydown.enter="activate($event, element)"
          >
            <title>{{ displayName(element) }} - {{ displayCount(element) }} items</title>
            <rect
              :x="element.x * canvasWidth"
              :y="element.y * canvasHeight"
              :width="element.width * canvasWidth"
              :height="element.height * canvasHeight"
              rx="5"
              class="fill-card stroke-foreground/70 transition-colors hover:fill-accent"
              :class="{ '!stroke-primary': element.id === selectedId }"
              stroke-width="4"
            />
            <text
              :x="(element.x + element.width / 2) * canvasWidth"
              :y="(element.y + element.height / 2) * canvasHeight - 5"
              text-anchor="middle"
              :clip-path="`url(#location-layout-clip-${element.id})`"
              class="pointer-events-none fill-foreground text-[20px] font-semibold"
            >
              {{ displayName(element) }}
            </text>
            <text
              :x="(element.x + element.width / 2) * canvasWidth"
              :y="(element.y + element.height / 2) * canvasHeight + 22"
              text-anchor="middle"
              :clip-path="`url(#location-layout-clip-${element.id})`"
              class="pointer-events-none fill-muted-foreground text-[16px]"
            >
              {{ displayCount(element) }} items
            </text>
          </g>
        </template>

        <template v-if="editable && tool === 'select' && selected">
          <g v-if="selected.kind === 'wall'">
            <circle
              :cx="selected.x * canvasWidth"
              :cy="selected.y * canvasHeight"
              r="12"
              class="cursor-crosshair fill-primary stroke-background"
              stroke-width="4"
              @pointerdown="startHandle($event, 'wall-start')"
            />
            <circle
              :cx="selected.endX * canvasWidth"
              :cy="selected.endY * canvasHeight"
              r="12"
              class="cursor-crosshair fill-primary stroke-background"
              stroke-width="4"
              @pointerdown="startHandle($event, 'wall-end')"
            />
          </g>
          <g
            v-else
            :transform="`rotate(${selected.rotation} ${(selected.x + selected.width / 2) * canvasWidth} ${(selected.y + selected.height / 2) * canvasHeight})`"
          >
            <line
              :x1="(selected.x + selected.width / 2) * canvasWidth"
              :y1="selected.y * canvasHeight"
              :x2="(selected.x + selected.width / 2) * canvasWidth"
              :y2="selected.y * canvasHeight - 38"
              class="stroke-primary"
              stroke-width="3"
            />
            <circle
              data-layout-handle="rotate"
              :cx="(selected.x + selected.width / 2) * canvasWidth"
              :cy="selected.y * canvasHeight - 45"
              r="11"
              class="cursor-alias fill-primary stroke-background"
              stroke-width="4"
              @pointerdown="startHandle($event, 'rotate')"
            />
            <rect
              data-layout-handle="resize"
              :x="(selected.x + selected.width) * canvasWidth - 10"
              :y="(selected.y + selected.height) * canvasHeight - 10"
              width="20"
              height="20"
              class="cursor-nwse-resize fill-primary stroke-background"
              stroke-width="4"
              @pointerdown="startHandle($event, 'resize')"
            />
          </g>
        </template>
      </g>
    </svg>
  </div>
</template>
