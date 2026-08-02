import type { APIRequestContext } from "@playwright/test";
import { devices, expect, request as playwrightRequest, test } from "@playwright/test";

type Entity = {
  id: string;
  name: string;
};

type EntityType = {
  id: string;
  isLocation: boolean;
};

type Layout = {
  revision: number;
  walls: Array<{
    id: string;
    x: number;
    y: number;
    endX: number;
    endY: number;
    zOrder: number;
  }>;
  locations: Array<{
    id: string;
    targetId: string;
    x: number;
    y: number;
    width: number;
    height: number;
    rotation: number;
    zOrder: number;
  }>;
};

const hasConfiguredCredentials = Boolean(process.env.E2E_EMAIL && process.env.E2E_PASSWORD);
const email = process.env.E2E_EMAIL || `layout-e2e-${process.pid}-${Date.now()}@example.com`;
const password = process.env.E2E_PASSWORD || "LayoutE2eTemporaryPassword2026";

test.setTimeout(60_000);

async function login(request: APIRequestContext) {
  let response = await request.post("/api/v1/users/login", {
    data: { username: email, password, stayLoggedIn: false },
  });

  if (!response.ok() && !hasConfiguredCredentials) {
    const registration = await request.post("/api/v1/users/register", {
      data: {
        email,
        name: "Layout E2E",
        password,
        token: "",
      },
    });
    expect(registration.status()).toBe(204);
    response = await request.post("/api/v1/users/login", {
      data: { username: email, password, stayLoggedIn: false },
    });
  }

  expect(response.ok()).toBeTruthy();
}

async function locationType(request: APIRequestContext): Promise<EntityType> {
  const response = await request.get("/api/v1/entity-types");
  expect(response.ok()).toBeTruthy();
  const types = (await response.json()) as EntityType[];
  const location = types.find(type => type.isLocation);
  expect(location).toBeTruthy();
  return location!;
}

async function createLocation(request: APIRequestContext, typeId: string, name: string, parentId?: string) {
  const response = await request.post("/api/v1/entities", {
    data: {
      aliases: [],
      description: "",
      entityTypeId: typeId,
      name,
      parentId,
      quantity: 1,
      tagIds: [],
    },
  });
  expect(response.ok()).toBeTruthy();
  return (await response.json()) as Entity;
}

async function deleteEntities(entities: Array<Entity | undefined>) {
  const request = await playwrightRequest.newContext({
    baseURL: process.env.E2E_BASE_URL || "http://localhost:3000",
  });
  try {
    await login(request);
    for (const entity of entities) {
      if (entity) await request.delete(`/api/v1/entities/${entity.id}`);
    }
  } finally {
    await request.dispose();
  }
}

function inputElements(layout: Layout) {
  return [
    ...layout.walls.map(wall => ({
      ...wall,
      kind: "wall",
      width: 0,
      height: 0,
      rotation: 0,
    })),
    ...layout.locations.map(location => ({
      ...location,
      kind: "location",
      endX: 0,
      endY: 0,
    })),
  ];
}

test("draws, places, edits, saves, and detects a revision conflict", async ({ page }, testInfo) => {
  await login(page.request);
  const type = await locationType(page.request);
  const suffix = Date.now().toString(36);
  let room: Entity | undefined;
  let shelf: Entity | undefined;
  let cabinet: Entity | undefined;
  let cart: Entity | undefined;

  try {
    room = await createLocation(page.request, type.id, `Layout Room ${suffix}`);
    shelf = await createLocation(page.request, type.id, `Shelf ${suffix}`, room.id);
    cabinet = await createLocation(page.request, type.id, `Cabinet ${suffix}`, room.id);
    cart = await createLocation(page.request, type.id, `Unplaced Cart ${suffix}`, room.id);

    await page.goto(`/location/${room.id}`);
    const section = page.getByTestId("location-layout-section");
    await expect(section).toBeVisible({ timeout: 20_000 });
    await section.getByRole("button", { name: "Overhead" }).click();
    await section.getByRole("button", { name: "Create layout" }).click();

    const editor = page.getByTestId("location-layout-editor");
    const canvas = editor.getByTestId("location-layout-canvas");
    const svg = canvas.getByRole("img", { name: "Overhead location layout" });
    await editor.getByTitle("Wall", { exact: true }).click();

    await svg.scrollIntoViewIfNeeded();
    const box = await svg.boundingBox();
    expect(box).toBeTruthy();
    const point = (x: number, y: number) => ({
      x: box!.x + box!.width * x,
      y: box!.y + box!.height * y,
    });
    const walls: Array<[ReturnType<typeof point>, ReturnType<typeof point>]> = [
      [point(0.12, 0.15), point(0.82, 0.11)],
      [point(0.82, 0.11), point(0.9, 0.48)],
      [point(0.9, 0.48), point(0.72, 0.84)],
      [point(0.72, 0.84), point(0.14, 0.76)],
    ];
    for (const [start, end] of walls) {
      await page.mouse.move(start.x, start.y);
      await page.mouse.down();
      await page.mouse.move(end.x, end.y, { steps: 4 });
      await page.mouse.up();
    }
    await expect(svg.locator('[data-layout-kind="wall"]')).toHaveCount(4);

    await editor.getByTitle("Select", { exact: true }).click();
    await editor.getByRole("button", { name: shelf.name }).dragTo(canvas, {
      targetPosition: { x: box!.width * 0.44, y: box!.height * 0.42 },
    });
    await editor.getByRole("button", { name: cabinet.name }).dragTo(canvas, {
      targetPosition: { x: box!.width * 0.52, y: box!.height * 0.48 },
    });
    await expect(svg.locator('[data-layout-kind="location"]')).toHaveCount(2);
    await expect(editor.getByRole("button", { name: cart.name })).toBeVisible();

    const topPlacement = svg.locator(`[data-target-id="${cabinet.id}"]`);
    await topPlacement.click();
    const rotate = svg.locator('[data-layout-handle="rotate"]');
    const rotateBox = await rotate.boundingBox();
    expect(rotateBox).toBeTruthy();
    await page.mouse.move(rotateBox!.x + rotateBox!.width / 2, rotateBox!.y + rotateBox!.height / 2);
    await page.mouse.down();
    await page.mouse.move(rotateBox!.x + 55, rotateBox!.y + 35, { steps: 5 });
    await page.mouse.up();

    await editor.getByTitle("Undo").click();
    await expect(editor.getByTitle("Redo")).toBeEnabled();
    await editor.getByTitle("Redo").click();

    const saveResponse = page.waitForResponse(
      response =>
        response.url().endsWith(`/api/v1/entities/${room!.id}/layout`) && response.request().method() === "PUT"
    );
    await editor.getByRole("button", { name: "Save" }).click();
    expect((await saveResponse).status()).toBe(200);
    await expect(editor).toHaveCount(0);
    await expect(section.locator('[data-layout-kind="wall"]')).toHaveCount(4);
    await expect(section.locator('[data-layout-kind="location"]')).toHaveCount(2);
    await expect(section.getByText(cart.name)).toBeVisible();

    await testInfo.attach("desktop-overhead-layout", {
      body: await section.screenshot(),
      contentType: "image/png",
    });

    await section.getByRole("button", { name: "Edit layout" }).click();
    const currentResponse = await page.request.get(`/api/v1/entities/${room.id}/layout`);
    const current = (await currentResponse.json()) as Layout;
    const concurrent = await page.request.put(`/api/v1/entities/${room.id}/layout`, {
      data: {
        expectedRevision: current.revision,
        elements: inputElements(current),
      },
    });
    expect(concurrent.status()).toBe(200);

    const conflictResponse = page.waitForResponse(
      response =>
        response.url().endsWith(`/api/v1/entities/${room!.id}/layout`) && response.request().method() === "PUT"
    );
    await page.getByTestId("location-layout-editor").getByRole("button", { name: "Save" }).click();
    expect((await conflictResponse).status()).toBe(409);
    await expect(page.getByText("This layout changed in another session.")).toBeVisible();
    await expect(page.getByTestId("location-layout-editor")).toHaveCount(0);

    await section.locator(`[data-target-id="${cabinet.id}"]`).click();
    await expect(page).toHaveURL(new RegExp(`/location/${cabinet.id}$`));
  } finally {
    await page.close();
    await deleteEntities([cart, cabinet, shelf, room]);
  }
});

test.describe("mobile overhead view", () => {
  const pixel = devices["Pixel 7"];
  test.use({
    deviceScaleFactor: pixel.deviceScaleFactor,
    hasTouch: pixel.hasTouch,
    isMobile: pixel.isMobile,
    userAgent: pixel.userAgent,
    viewport: pixel.viewport,
  });

  test("shows an existing layout without editor controls and supports navigation", async ({ page }, testInfo) => {
    await login(page.request);
    const type = await locationType(page.request);
    const suffix = Date.now().toString(36);
    let room: Entity | undefined;
    let shelf: Entity | undefined;
    let cabinet: Entity | undefined;

    try {
      room = await createLocation(page.request, type.id, `Mobile Room ${suffix}`);
      shelf = await createLocation(page.request, type.id, `Mobile Shelf ${suffix}`, room.id);
      cabinet = await createLocation(page.request, type.id, `Mobile Cabinet ${suffix}`, room.id);
      const layoutResponse = await page.request.put(`/api/v1/entities/${room.id}/layout`, {
        data: {
          expectedRevision: 0,
          elements: [
            {
              kind: "wall",
              x: 0.08,
              y: 0.1,
              endX: 0.88,
              endY: 0.14,
              zOrder: 0,
            },
            {
              kind: "wall",
              x: 0.88,
              y: 0.14,
              endX: 0.82,
              endY: 0.82,
              zOrder: 1,
            },
            {
              kind: "wall",
              x: 0.82,
              y: 0.82,
              endX: 0.16,
              endY: 0.7,
              zOrder: 2,
            },
            {
              kind: "location",
              targetId: shelf.id,
              x: 0.3,
              y: 0.3,
              width: 0.28,
              height: 0.18,
              rotation: 25,
              zOrder: 3,
            },
            {
              kind: "location",
              targetId: cabinet.id,
              x: 0.42,
              y: 0.38,
              width: 0.26,
              height: 0.2,
              rotation: -18,
              zOrder: 4,
            },
          ],
        },
      });
      expect(layoutResponse.status()).toBe(200);

      await page.goto(`/location/${room.id}`);
      const section = page.getByTestId("location-layout-section");
      await expect(section).toBeVisible({ timeout: 20_000 });
      await expect(section.locator('[data-layout-kind="location"]')).toHaveCount(2, { timeout: 20_000 });
      await expect(section.getByRole("button", { name: "Edit layout" })).toHaveCount(0);
      await expect(section.getByRole("button", { name: "Create layout" })).toHaveCount(0);

      const before = await section.locator("svg > g").getAttribute("transform");
      await section.getByTitle("Zoom in").click();
      const after = await section.locator("svg > g").getAttribute("transform");
      expect(after).not.toBe(before);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);

      await testInfo.attach("mobile-overhead-layout", {
        body: await section.screenshot(),
        contentType: "image/png",
      });

      await section.locator(`[data-target-id="${cabinet.id}"]`).click();
      await expect(page).toHaveURL(new RegExp(`/location/${cabinet.id}$`));
    } finally {
      await page.close();
      await deleteEntities([cabinet, shelf, room]);
    }
  });
});
