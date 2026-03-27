import { Box, CircularProgress, Typography, useTheme } from '@mui/material'
import { useState } from 'react'
import { useParams } from 'react-router-dom'

import { externalPages } from '../constants/externalPages'
import { getExternalPageEndpoint } from '../utils/endpoints'

const ExternalPageViewer = () => {
  const { pageSlug } = useParams<{ pageSlug: string }>()
  const [loading, setLoading] = useState(true)
  const theme = useTheme()
  const page = externalPages.find((p) => p.slug === pageSlug)

  if (!page) {
    return (
      <Typography sx={{ p: 4 }} variant="h6">
        Page not found.
      </Typography>
    )
  }

  return (
    <Box
      sx={{ height: 'calc(100vh - 64px)', position: 'relative' }}
      data-tour="external-page-content"
    >
      {loading && (
        <Box
          sx={{ display: 'flex', justifyContent: 'center', pt: 8, position: 'absolute', inset: 0 }}
        >
          <CircularProgress />
        </Box>
      )}
      <iframe
        src={`${getExternalPageEndpoint(page.slug)}?theme=${theme.palette.mode}`}
        title={page.label}
        onLoad={() => setLoading(false)}
        style={{
          width: '100%',
          height: '100%',
          border: 'none',
          visibility: loading ? 'hidden' : 'visible',
        }}
      />
    </Box>
  )
}

export default ExternalPageViewer
