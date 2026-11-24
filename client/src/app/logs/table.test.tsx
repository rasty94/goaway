import { flexRender, useTable } from "@tanstack/react-table";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Queries } from "./columns";
import { columns } from "./columnsData";
import { logsTableFeatures } from "./tableFeatures";

const row: Queries = {
  blocked: false,
  cached: true,
  client: { ip: "192.0.2.1", name: "laptop", mac: "" },
  domain: "example.com",
  ip: [{ ip: "93.184.216.34", rtype: "A" }],
  queryType: "A",
  responseTimeNS: 1_000_000,
  status: "NOERROR",
  timestamp: "2026-11-25T00:00:00Z",
  protocol: "UDP"
};

function LogsTable() {
  const table = useTable({
    features: logsTableFeatures,
    data: [row],
    columns,
    manualPagination: true,
    manualSorting: true,
    pageCount: 1,
    state: {
      sorting: [{ id: "timestamp", desc: true }],
      columnVisibility: { protocol: false },
      pagination: { pageIndex: 0, pageSize: 10 }
    }
  });

  const hideable = table
    .getAllColumns()
    .filter((c) => c.getCanHide())
    .map((c) => `${c.id}:${c.getIsVisible()}`);

  return (
    <table data-hideable={hideable.join(",")}>
      <thead>
        {table.getHeaderGroups().map((hg) => (
          <tr key={hg.id}>
            {hg.headers.map((h) => (
              <th key={h.id}>
                {flexRender(h.column.columnDef.header, h.getContext())}
              </th>
            ))}
          </tr>
        ))}
      </thead>
      <tbody data-count={table.getPreSelectedRowModel().rows.length}>
        {table.getRowModel().rows.map((r) => (
          <tr key={r.id}>
            {r.getVisibleCells().map((c) => (
              <td key={c.id} data-col={c.column.id}>
                {flexRender(c.column.columnDef.cell, c.getContext())}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

describe("logs table (react-table v9)", () => {
  it("renders headers, rows and respects column visibility", () => {
    const html = renderToStaticMarkup(<LogsTable />);
    expect(html).toContain("example.com");
    expect(html).toContain('data-count="1"');
    expect(html).toContain("protocol:false");
    expect(html).not.toContain('data-col="protocol"');
    expect(html).toContain('data-col="domain"');
  });
});
