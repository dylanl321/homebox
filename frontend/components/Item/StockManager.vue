<script setup lang="ts">
  import { toast } from "@/components/ui/sonner";
  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Badge } from "@/components/ui/badge";
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
  import { Label } from "@/components/ui/label";
  import { Separator } from "@/components/ui/separator";
  import { Check, History, Minus, Plus, Star, Trash2 } from "lucide-vue-next";
  import { useI18n } from "vue-i18n";
  import LocationSelector from "@/components/Location/Selector.vue";
  import DateTime from "@/components/global/DateTime.vue";
  import type { EntitySummary } from "~~/lib/api/types/data-contracts";
  import type { StockAllocation, StockOperation, StockState, StockTransaction } from "~~/lib/api/types/stock";

  const props = defineProps<{
    itemId: string;
    initialStock?: StockState;
  }>();

  const emit = defineEmits<{
    updated: [stock: StockState];
  }>();

  const { t } = useI18n();
  const api = useUserApi();
  const stock = ref<StockState>();
  const history = ref<StockTransaction[]>([]);
  const loading = ref(false);
  const historyLoading = ref(false);
  const newLocation = ref<EntitySummary | null>(null);
  const newQuantity = ref(1);
  const exactQuantities = reactive<Record<string, number>>({});
  const transferSource = ref<StockAllocation>();
  const transferDestination = ref<EntitySummary | null>(null);
  const transferQuantity = ref(1);
  const locations = useFlatLocations();

  const positiveAllocations = computed(
    () => stock.value?.allocations.filter(allocation => allocation.quantity > 0) ?? []
  );

  function allocationKey(allocation: StockAllocation) {
    return allocation.locationId ?? "unassigned";
  }

  function locationName(allocation: StockAllocation) {
    return allocation.location?.name || t("stock.unassigned");
  }

  function idempotencyKey() {
    return crypto.randomUUID();
  }

  function syncDrafts(next: StockState) {
    for (const allocation of next.allocations) {
      exactQuantities[allocationKey(allocation)] = allocation.quantity;
    }
  }

  async function loadStock() {
    loading.value = true;
    const response = await api.items.getStock(props.itemId);
    loading.value = false;
    if (response.error) {
      toast.error(t("stock.toast.load_failed"));
      return;
    }
    stock.value = response.data;
    syncDrafts(response.data);
  }

  async function loadHistory() {
    historyLoading.value = true;
    const response = await api.items.getStockTransactions({
      entityId: props.itemId,
      page: 1,
      pageSize: 25,
    });
    historyLoading.value = false;
    if (response.error) {
      toast.error(t("stock.toast.history_failed"));
      return;
    }
    history.value = response.data.items;
  }

  async function submit(operation: StockOperation) {
    loading.value = true;
    const response = await api.items.updateStock(props.itemId, operation);
    loading.value = false;
    if (response.error) {
      toast.error(response.status === 409 ? t("stock.toast.conflict") : t("stock.toast.update_failed"));
      return false;
    }
    stock.value = response.data;
    syncDrafts(response.data);
    emit("updated", response.data);
    await loadHistory();
    toast.success(t("stock.toast.updated"));
    return true;
  }

  async function adjust(allocation: StockAllocation, delta: number) {
    await submit({
      operation: "adjust",
      locationId: allocation.locationId ?? null,
      delta,
      workflow: "web",
      idempotencyKey: idempotencyKey(),
    });
  }

  async function setQuantity(allocation: StockAllocation, requestedQuantity?: number) {
    const quantity = requestedQuantity ?? exactQuantities[allocationKey(allocation)];
    if (quantity === undefined || !Number.isFinite(quantity) || quantity < 0) {
      toast.error(t("stock.toast.invalid_quantity"));
      return;
    }
    await submit({
      operation: "set",
      locationId: allocation.locationId ?? null,
      quantity,
      workflow: "web",
      idempotencyKey: idempotencyKey(),
    });
  }

  async function addAllocation() {
    if (!newLocation.value || !Number.isFinite(newQuantity.value) || newQuantity.value <= 0) {
      toast.error(t("stock.toast.location_quantity_required"));
      return;
    }
    const succeeded = await submit({
      operation: "adjust",
      locationId: newLocation.value.id,
      delta: newQuantity.value,
      workflow: "web",
      reason: t("stock.reason.add_allocation"),
      idempotencyKey: idempotencyKey(),
    });
    if (succeeded) {
      newLocation.value = null;
      newQuantity.value = 1;
    }
  }

  async function transfer() {
    if (
      !transferSource.value ||
      !transferDestination.value ||
      transferDestination.value.id === transferSource.value.locationId ||
      !Number.isFinite(transferQuantity.value) ||
      transferQuantity.value <= 0
    ) {
      toast.error(t("stock.toast.transfer_invalid"));
      return;
    }
    const succeeded = await submit({
      operation: "transfer",
      fromLocationId: transferSource.value.locationId ?? null,
      toLocationId: transferDestination.value.id,
      quantity: transferQuantity.value,
      workflow: "web",
      idempotencyKey: idempotencyKey(),
    });
    if (succeeded) {
      transferSource.value = undefined;
      transferDestination.value = null;
      transferQuantity.value = 1;
    }
  }

  async function setDefault(allocation: StockAllocation) {
    loading.value = true;
    const response = await api.items.setDefaultStockLocation(props.itemId, allocation.locationId ?? null);
    loading.value = false;
    if (response.error) {
      toast.error(t("stock.toast.default_failed"));
      return;
    }
    stock.value = response.data;
    syncDrafts(response.data);
    emit("updated", response.data);
    toast.success(t("stock.toast.default_updated"));
  }

  function transactionLocation(transaction: StockTransaction) {
    const source =
      transaction.sourceLocation?.name ||
      locations.value.find(location => location.id === transaction.sourceLocationId)?.name;
    const destination =
      transaction.destinationLocation?.name ||
      locations.value.find(location => location.id === transaction.destinationLocationId)?.name;
    if (source && destination) return t("stock.transfer_path", { source, destination });
    return destination || source || t("stock.unassigned");
  }

  watch(
    () => props.initialStock,
    value => {
      if (value) {
        stock.value = value;
        syncDrafts(value);
      }
    },
    { immediate: true }
  );

  onMounted(async () => {
    if (!stock.value) await loadStock();
    await loadHistory();
  });
</script>

<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <p class="text-sm text-muted-foreground">
          {{ $t("stock.total_quantity") }}
        </p>
        <p class="text-2xl font-semibold">{{ stock?.totalQuantity ?? 0 }}</p>
      </div>
      <Badge variant="secondary">
        {{ $t("stock.location_count", { count: positiveAllocations.length }) }}
      </Badge>
    </div>

    <div class="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ $t("global.location") }}</TableHead>
            <TableHead class="w-48">{{ $t("global.quantity") }}</TableHead>
            <TableHead class="min-w-72 text-right">{{ $t("stock.actions") }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="allocation in positiveAllocations" :key="allocation.id">
            <TableCell>
              <div class="flex items-center gap-2">
                <NuxtLink
                  v-if="allocation.locationId"
                  :to="`/location/${allocation.locationId}`"
                  class="font-medium hover:underline"
                >
                  {{ locationName(allocation) }}
                </NuxtLink>
                <span v-else>{{ locationName(allocation) }}</span>
                <Badge v-if="allocation.isDefault" variant="outline">{{ $t("stock.default") }}</Badge>
              </div>
            </TableCell>
            <TableCell>
              <div class="flex min-w-40 items-center gap-1">
                <Button
                  size="icon"
                  variant="outline"
                  :aria-label="$t('stock.decrease')"
                  :disabled="loading"
                  @click="adjust(allocation, -1)"
                >
                  <Minus class="size-4" />
                </Button>
                <Input
                  v-model.number="exactQuantities[allocationKey(allocation)]"
                  class="w-24 text-right"
                  type="number"
                  min="0"
                  step="any"
                  :aria-label="$t('stock.exact_quantity')"
                  @keyup.enter="setQuantity(allocation)"
                />
                <Button
                  size="icon"
                  variant="outline"
                  :aria-label="$t('stock.increase')"
                  :disabled="loading"
                  @click="adjust(allocation, 1)"
                >
                  <Plus class="size-4" />
                </Button>
              </div>
            </TableCell>
            <TableCell>
              <div class="flex justify-end gap-1">
                <Button size="sm" variant="outline" :disabled="loading" @click="setQuantity(allocation)">
                  <Check class="size-4" />
                  {{ $t("stock.set") }}
                </Button>
                <Button size="sm" variant="outline" :disabled="loading" @click="transferSource = allocation">
                  {{ $t("stock.transfer") }}
                </Button>
                <Button
                  v-if="!allocation.isDefault"
                  size="icon"
                  variant="ghost"
                  :title="$t('stock.set_default')"
                  :aria-label="$t('stock.set_default')"
                  :disabled="loading"
                  @click="setDefault(allocation)"
                >
                  <Star class="size-4" />
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  :title="$t('stock.remove')"
                  :aria-label="$t('stock.remove')"
                  :disabled="loading"
                  @click="setQuantity(allocation, 0)"
                >
                  <Trash2 class="size-4 text-destructive" />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <div v-if="transferSource" class="grid gap-3 border-l-4 border-primary pl-4 md:grid-cols-[1fr_10rem_auto]">
      <div>
        <Label>{{ $t("stock.transfer_destination") }}</Label>
        <LocationSelector v-model="transferDestination" />
      </div>
      <div>
        <Label for="stock-transfer-quantity">{{ $t("global.quantity") }}</Label>
        <Input id="stock-transfer-quantity" v-model.number="transferQuantity" type="number" min="0" step="any" />
      </div>
      <div class="flex items-end gap-2">
        <Button :disabled="loading" @click="transfer">{{ $t("stock.transfer") }}</Button>
        <Button variant="outline" @click="transferSource = undefined">{{ $t("global.cancel") }}</Button>
      </div>
    </div>

    <Separator />

    <div class="grid gap-3 md:grid-cols-[1fr_10rem_auto]">
      <LocationSelector v-model="newLocation" />
      <div>
        <Label for="stock-new-quantity">{{ $t("global.quantity") }}</Label>
        <Input id="stock-new-quantity" v-model.number="newQuantity" type="number" min="0" step="any" />
      </div>
      <Button class="self-end" :disabled="loading" @click="addAllocation">
        <Plus class="size-4" />
        {{ $t("stock.add_allocation") }}
      </Button>
    </div>

    <Separator />

    <section>
      <h3 class="mb-3 flex items-center gap-2 text-base font-semibold">
        <History class="size-4" />
        {{ $t("stock.history") }}
      </h3>
      <p v-if="historyLoading" class="text-sm text-muted-foreground">
        {{ $t("global.loading") }}
      </p>
      <p v-else-if="history.length === 0" class="text-sm text-muted-foreground">
        {{ $t("stock.no_history") }}
      </p>
      <div v-else class="divide-y rounded-md border">
        <div v-for="transaction in history" :key="transaction.id" class="grid gap-1 px-3 py-2 sm:grid-cols-4">
          <span class="font-medium">{{ $t(`stock.operation.${transaction.operation}`) }}</span>
          <span>{{ transactionLocation(transaction) }}</span>
          <span>
            {{ transaction.quantity }}
            {{
              $t("stock.quantity_change", {
                before: transaction.beforeTotal,
                after: transaction.afterTotal,
              })
            }}
          </span>
          <span class="text-sm text-muted-foreground sm:text-right">
            <DateTime :date="transaction.createdAt" />
          </span>
          <p v-if="transaction.reason" class="text-sm text-muted-foreground sm:col-span-4">
            {{ transaction.reason }}
          </p>
        </div>
      </div>
    </section>
  </div>
</template>
