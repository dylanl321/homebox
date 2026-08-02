import { describe, expect, it } from "vitest";
import type { LocationLayoutElementInput } from "~~/lib/api/types/data-contracts";
import {
  commitHistory,
  createPlacement,
  elementsForSave,
  emptyElement,
  isRevisionConflict,
  normalizeRotation,
  redoHistory,
  snapPoint,
  undoHistory,
  unplacedChildren,
} from "./layout";

describe("location layout geometry", () => {
  it("normalizes rotation into the API range", () => {
    expect(normalizeRotation(270)).toBe(-90);
    expect(normalizeRotation(-540)).toBe(-180);
  });

  it("snaps wall points to the grid", () => {
    expect(snapPoint({ x: 0.111, y: 0.139 }, [], true, false)).toEqual({
      x: 0.1,
      y: 0.15,
    });
  });

  it("prefers nearby wall endpoints when enabled", () => {
    const wall = {
      ...emptyElement("wall"),
      x: 0.1,
      y: 0.1,
      endX: 0.4,
      endY: 0.4,
    };
    expect(snapPoint({ x: 0.405, y: 0.405 }, [wall], false, true)).toEqual({
      x: 0.4,
      y: 0.4,
    });
  });

  it("places locations inside the canvas and reports unplaced children", () => {
    const placement = createPlacement("shelf-a", { x: 0.99, y: 0.99 }, 2);
    expect(placement.x + placement.width).toBe(1);
    expect(placement.y + placement.height).toBe(1);
    const children = [
      { id: "shelf-a", name: "Shelf A" },
      { id: "cabinet-b", name: "Cabinet B" },
    ];
    expect(unplacedChildren(children as never[], [placement]).map(child => child.id)).toEqual(["cabinet-b"]);
  });

  it("tracks undo and redo snapshots", () => {
    const first: LocationLayoutElementInput[] = [{ ...emptyElement("wall"), x: 0.1 }];
    const second: LocationLayoutElementInput[] = [{ ...first[0]!, x: 0.2 }];
    const past: (typeof first)[] = [];
    const future: (typeof first)[] = [];
    commitHistory(past, future, first, second);
    expect(undoHistory(past, future, second)[0]!.x).toBe(0.1);
    expect(redoHistory(past, future, first)[0]!.x).toBe(0.2);
  });

  it("omits wall targets when serializing the save payload", () => {
    const wall = { ...emptyElement("wall"), targetId: "" };
    const location = { ...emptyElement("location"), targetId: "shelf-a" };
    const serialized = elementsForSave([wall, location]);
    expect(serialized[0]!.targetId).toBeUndefined();
    expect(serialized[1]!.targetId).toBe("shelf-a");
    expect(serialized.map(element => element.zOrder)).toEqual([0, 1]);
  });

  it("identifies optimistic save conflicts", () => {
    expect(isRevisionConflict(409)).toBe(true);
    expect(isRevisionConflict(400)).toBe(false);
  });
});
