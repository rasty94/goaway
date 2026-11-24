import {
  columnFilteringFeature,
  columnVisibilityFeature,
  rowPaginationFeature,
  rowSelectionFeature,
  rowSortingFeature,
  tableFeatures
} from "@tanstack/react-table";

// Sorting and pagination are server-side (manual*), so no client row models are needed.
export const logsTableFeatures = tableFeatures({
  rowSortingFeature,
  columnFilteringFeature,
  columnVisibilityFeature,
  rowSelectionFeature,
  rowPaginationFeature
});
