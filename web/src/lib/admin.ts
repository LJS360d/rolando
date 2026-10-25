import { computed, signal } from "@preact/signals";
import { resource, type AdminPage, type Resource } from "./api";

export const adminPath = signal("/admin/guilds?sort=bytes&dir=desc&page=1&q=");

let last: Resource<AdminPage> = {};
// adminPage keeps the last settled result while the next path loads, so islands keep their content (and stats, being path-independent, don't rerender).
export const adminPage = computed(() => {
  const r = resource<AdminPage>(adminPath.value).value;
  return (last = r.data || r.error ? r : last);
});
export const adminLoading = computed(() => {
  const r = resource<AdminPage>(adminPath.value).value;
  return !r.data && !r.error;
});
