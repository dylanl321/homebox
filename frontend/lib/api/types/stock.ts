import type { QueryValue } from "../base/urls";
import type {
  EntityOut,
  EntitySummary,
  PaginationResultStockTransaction,
  StockAllocation as GeneratedStockAllocation,
  StockLocationSummary as GeneratedStockLocationSummary,
  StockState as GeneratedStockState,
  StockTransaction as GeneratedStockTransaction,
} from "./data-contracts";

export type StockAllocation = GeneratedStockAllocation;
export type StockLocationSummary = GeneratedStockLocationSummary;
export type StockState = GeneratedStockState;
export type StockTransaction = GeneratedStockTransaction;

export type StockEntityOut = EntityOut & {
  stock?: StockState;
};

export type StockEntitySummary = EntitySummary & {
  location?: EntitySummary | null;
  locationCount?: number;
  allocatedQuantity?: number | null;
};

interface StockOperationBase {
  idempotencyKey: string;
  workflow?: string;
  reason?: string;
}

export interface StockAdjustOperation extends StockOperationBase {
  operation: "adjust";
  locationId: string | null;
  delta: number;
}

export interface StockSetOperation extends StockOperationBase {
  operation: "set";
  locationId: string | null;
  quantity: number;
}

export interface StockTransferOperation extends StockOperationBase {
  operation: "transfer";
  fromLocationId: string | null;
  toLocationId: string | null;
  quantity: number;
}

export type StockOperation = StockAdjustOperation | StockSetOperation | StockTransferOperation;

export type StockTransactionList = PaginationResultStockTransaction;

export interface StockTransactionQuery {
  [key: string]: QueryValue;
  entityId?: string;
  locationId?: string;
  page?: number;
  pageSize?: number;
}

export interface LocationStockResolution {
  locationId: string;
  itemCount: number;
  totalQuantity: number;
  allocations: Array<{
    entityId: string;
    entityName: string;
    quantity: number;
  }>;
}

interface LocationStockResolutionBase {
  idempotencyKey: string;
  workflow?: string;
  reason?: string;
}

export type LocationStockResolutionRequest =
  | (LocationStockResolutionBase & { action: "transfer"; destinationLocationId: string })
  | (LocationStockResolutionBase & { action: "remove"; confirmed: true });

export interface StockApiError {
  code?: string;
  error?: string;
}
