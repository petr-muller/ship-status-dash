import type { SubComponentListParams } from '../types'

import { slugify } from './slugify'

export const getPublicDomain = () => {
  const envDomain = import.meta.env.VITE_PUBLIC_DOMAIN
  if (!envDomain) {
    throw new Error('VITE_PUBLIC_DOMAIN environment variable is required')
  }
  return envDomain
}

export const getProtectedDomain = () => {
  const envDomain = import.meta.env.VITE_PROTECTED_DOMAIN
  if (!envDomain) {
    throw new Error('VITE_PROTECTED_DOMAIN environment variable is required')
  }
  return envDomain
}

export const getComponentsEndpoint = () => `${getPublicDomain()}/api/components`

export const getTagsEndpoint = () => `${getPublicDomain()}/api/tags`

export const getComponentInfoEndpoint = (componentName: string) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}`

export const getOverallStatusEndpoint = () => `${getPublicDomain()}/api/status`

export const getSubComponentStatusEndpoint = (componentName: string, subComponentName: string) =>
  `${getPublicDomain()}/api/status/${slugify(componentName)}/${slugify(subComponentName)}`

export const getComponentStatusEndpoint = (componentName: string) =>
  `${getPublicDomain()}/api/status/${slugify(componentName)}`

export const getListSubComponentsEndpoint = (params: SubComponentListParams = {}) => {
  const search = new URLSearchParams()
  if (params.componentName) search.set('componentName', params.componentName)
  if (params.tag) search.set('tag', params.tag)
  if (params.team) search.set('team', params.team)
  const q = search.toString()
  return `${getPublicDomain()}/api/sub-components${q ? `?${q}` : ''}`
}

export const createOutageEndpoint = (componentName: string, subComponentName: string) =>
  `${getProtectedDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages`

export const getSubComponentOutagesEndpoint = (componentName: string, subComponentName: string) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages`

export const modifyOutageEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getProtectedDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}`

export const getOutageEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}`

export const getOutageAuditLogsEndpoint = (
  componentName: string,
  subComponentName: string,
  outageId: number,
) =>
  `${getPublicDomain()}/api/components/${slugify(componentName)}/${slugify(subComponentName)}/outages/${outageId}/audit-logs`

export const getUserEndpoint = () => `${getProtectedDomain()}/api/user`

export const getExternalPageEndpoint = (slug: string) =>
  `${getPublicDomain()}/api/external-pages/${slug}`
