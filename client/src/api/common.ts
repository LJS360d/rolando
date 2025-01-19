export interface PageMeta {
  page: number,
  pageSize: number,
  totalItems: number,
  totalPages: number
}

export interface Page<T> {
  data: T;
  meta: PageMeta;
}