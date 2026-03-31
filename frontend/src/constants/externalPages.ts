export const EXTERNAL_PAGES_PATH_PREFIX = '/pages'

export interface ExternalPage {
  label: string
  slug: string
  description?: string
}

export const externalPages: ExternalPage[] = [
  {
    label: 'SPC Dashboard',
    slug: 'spc-dashboard',
    description: 'SHIP Statistical Process Controls (updates every ~8 hours)',
  },
]
