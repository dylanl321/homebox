import type {
  EntityOut,
  EntitySummary,
  LocationLayoutElementInput,
  LocationLayoutOut,
} from "~~/lib/api/types/data-contracts";

export type ChildLocation = EntityOut | EntitySummary;
export type LayoutTool = "select" | "pan" | "wall";
export type Point = { x: number; y: number };

export const GRID_SIZE = 0.025;
export const ENDPOINT_SNAP_DISTANCE = 0.018;

export function newElementId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `layout-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function emptyElement(kind: "wall" | "location"): LocationLayoutElementInput {
  return {
    id: newElementId(),
    kind,
    targetId: "",
    x: 0,
    y: 0,
    width: 0,
    height: 0,
    endX: 0,
    endY: 0,
    rotation: 0,
    zOrder: 0,
  };
}

export function elementsFromLayout(layout: LocationLayoutOut): LocationLayoutElementInput[] {
  const walls = layout.walls.map(wall => ({
    ...emptyElement("wall"),
    ...wall,
  }));
  const locations = layout.locations.map(location => ({
    ...emptyElement("location"),
    ...location,
    kind: "location",
  }));
  return [...walls, ...locations].sort((a, b) => a.zOrder - b.zOrder);
}

export function cloneElements(elements: LocationLayoutElementInput[]): LocationLayoutElementInput[] {
  return elements.map(element => ({ ...element }));
}

export function elementsForSave(elements: LocationLayoutElementInput[]): LocationLayoutElementInput[] {
  return elements.map((element, zOrder) => {
    const { targetId, ...geometry } = element;
    const payload = element.kind === "location" ? { ...geometry, targetId, zOrder } : { ...geometry, zOrder };
    return payload as LocationLayoutElementInput;
  });
}

export function createPlacement(targetId: string, point: Point, zOrder: number): LocationLayoutElementInput {
  const width = 0.18;
  const height = 0.12;
  return {
    ...emptyElement("location"),
    targetId,
    x: clamp(point.x - width / 2, 0, 1 - width),
    y: clamp(point.y - height / 2, 0, 1 - height),
    width,
    height,
    zOrder,
  };
}

export function unplacedChildren(children: ChildLocation[], elements: LocationLayoutElementInput[]): ChildLocation[] {
  const placed = new Set(elements.filter(element => element.kind === "location").map(element => element.targetId));
  return children.filter(child => !placed.has(child.id));
}

export function commitHistory(
  past: LocationLayoutElementInput[][],
  future: LocationLayoutElementInput[][],
  before: LocationLayoutElementInput[],
  current: LocationLayoutElementInput[]
) {
  if (JSON.stringify(before) === JSON.stringify(current)) return;
  past.push(cloneElements(before));
  future.splice(0);
}

export function undoHistory(
  past: LocationLayoutElementInput[][],
  future: LocationLayoutElementInput[][],
  current: LocationLayoutElementInput[]
): LocationLayoutElementInput[] {
  const previous = past.pop();
  if (!previous) return current;
  future.push(cloneElements(current));
  return cloneElements(previous);
}

export function redoHistory(
  past: LocationLayoutElementInput[][],
  future: LocationLayoutElementInput[][],
  current: LocationLayoutElementInput[]
): LocationLayoutElementInput[] {
  const next = future.pop();
  if (!next) return current;
  past.push(cloneElements(current));
  return cloneElements(next);
}

export function isRevisionConflict(status: number): boolean {
  return status === 409;
}

export function normalizeRotation(rotation: number): number {
  const normalized = (((rotation + 180) % 360) + 360) % 360;
  return normalized - 180;
}

export function clamp(value: number, min = 0, max = 1): number {
  return Math.min(max, Math.max(min, value));
}

export function snapPoint(
  point: Point,
  elements: LocationLayoutElementInput[],
  gridSnap: boolean,
  endpointSnap: boolean,
  ignoreId = ""
): Point {
  let result = { ...point };
  if (gridSnap) {
    result = {
      x: Number((Math.round(result.x / GRID_SIZE) * GRID_SIZE).toFixed(6)),
      y: Number((Math.round(result.y / GRID_SIZE) * GRID_SIZE).toFixed(6)),
    };
  }

  if (endpointSnap) {
    const endpoints = elements.flatMap(element =>
      element.kind === "wall" && element.id !== ignoreId
        ? [
            { x: element.x, y: element.y },
            { x: element.endX, y: element.endY },
          ]
        : []
    );
    let closest: Point | undefined;
    let closestDistance = ENDPOINT_SNAP_DISTANCE;
    for (const endpoint of endpoints) {
      const distance = Math.hypot(result.x - endpoint.x, result.y - endpoint.y);
      if (distance < closestDistance) {
        closest = endpoint;
        closestDistance = distance;
      }
    }
    if (closest) result = { ...closest };
  }

  return { x: clamp(result.x), y: clamp(result.y) };
}

export function locationName(child: ChildLocation, fallback = ""): string {
  return child.name || fallback;
}

export function locationItemCount(child: ChildLocation, fallback = 0): number {
  return "itemCount" in child && typeof child.itemCount === "number" ? child.itemCount : fallback;
}
