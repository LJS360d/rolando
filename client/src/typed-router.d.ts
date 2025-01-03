/* eslint-disable */
/* prettier-ignore */
// @ts-nocheck
// Generated by unplugin-vue-router. ‼️ DO NOT MODIFY THIS FILE ‼️
// It's recommended to commit this file.
// Make sure to add this file to your tsconfig.json file as an "includes" or "files" entry.

declare module 'vue-router/auto-routes' {
  import type {
    RouteRecordInfo,
    ParamValue,
    ParamValueOneOrMore,
    ParamValueZeroOrMore,
    ParamValueZeroOrOne,
  } from 'vue-router'

  /**
   * Route name map generated by unplugin-vue-router
   */
  export interface RouteNamedMap {
    '/': RouteRecordInfo<'/', '/', Record<never, never>, Record<never, never>>,
    '/admin/': RouteRecordInfo<'/admin/', '/admin', Record<never, never>, Record<never, never>>,
    '/admin/broadcast': RouteRecordInfo<'/admin/broadcast', '/admin/broadcast', Record<never, never>, Record<never, never>>,
    '/data/[guildId]': RouteRecordInfo<'/data/[guildId]', '/data/:guildId', { guildId: ParamValue<true> }, { guildId: ParamValue<false> }>,
    '/login': RouteRecordInfo<'/login', '/login', Record<never, never>, Record<never, never>>,
    '/privacy-policy': RouteRecordInfo<'/privacy-policy', '/privacy-policy', Record<never, never>, Record<never, never>>,
    '/terms-of-service': RouteRecordInfo<'/terms-of-service', '/terms-of-service', Record<never, never>, Record<never, never>>,
  }
}
