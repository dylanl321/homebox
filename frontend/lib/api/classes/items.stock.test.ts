import { afterEach, describe, expect, test, vi } from "vitest";
import { Requests } from "~~/lib/requests";
import { ItemsApi } from "./items";

function response(body: unknown = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

describe("stock API adapter", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  test("uses the frozen item stock routes", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ totalQuantity: 2, allocations: [] }));
    vi.stubGlobal("fetch", fetchMock);
    const api = new ItemsApi(new Requests(""), "");

    await api.getStock("item-1");
    await api.updateStock("item-1", {
      operation: "adjust",
      locationId: "location-1",
      delta: 2,
      workflow: "web",
      idempotencyKey: "request-1",
    });
    await api.setDefaultStockLocation("item-1", "location-1");

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/entities/item-1/stock");
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/v1/entities/item-1/stock");
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({
        operation: "adjust",
        locationId: "location-1",
        delta: 2,
        workflow: "web",
        idempotencyKey: "request-1",
      }),
    });
    expect(fetchMock.mock.calls[2]?.[0]).toBe("/api/v1/entities/item-1/stock/default");
  });

  test("uses the frozen history and location resolution routes", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ items: [] }));
    vi.stubGlobal("fetch", fetchMock);
    const api = new ItemsApi(new Requests(""), "");

    await api.getStockTransactions({ entityId: "item-1", page: 2 });
    await api.getLocationStockResolution("location-1");
    await api.resolveLocationStock("location-1", {
      action: "transfer",
      destinationLocationId: "location-2",
      idempotencyKey: "resolution-1",
      workflow: "web-location-delete",
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/stock-transactions?entityId=item-1&page=2");
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/v1/entities/location-1/stock-resolution");
    expect(fetchMock.mock.calls[2]?.[0]).toBe("/api/v1/entities/location-1/stock-resolution");
    expect(fetchMock.mock.calls[2]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({
        action: "transfer",
        destinationLocationId: "location-2",
        idempotencyKey: "resolution-1",
        workflow: "web-location-delete",
      }),
    });
  });
});
