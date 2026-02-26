import {
  CheckCircle,
  Error as ErrorIcon,
  OpenInNew,
  ReportProblem,
  Warning,
} from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Paper,
  styled,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material'
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid'
import { DataGrid } from '@mui/x-data-grid'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { useAuth } from '../../contexts/AuthContext'
import { useTags } from '../../contexts/TagsContext'
import type { ComponentStatus, Outage, SubComponent } from '../../types'
import {
  getComponentInfoEndpoint,
  getSubComponentOutagesEndpoint,
  getSubComponentStatusEndpoint,
} from '../../utils/endpoints'
import { formatStatusSeverityText, relativeTime } from '../../utils/helpers'
import { deslugify } from '../../utils/slugify'
import { getStatusTintStyles } from '../../utils/styles'
import OutageActions from '../outage/actions/OutageActions'
import UpsertOutageModal from '../outage/actions/UpsertOutageModal'
import OutageDetailsButton from '../outage/OutageDetailsButton'
import { SeverityChip } from '../StatusColors'
import TagChip from '../tags/TagChip'

const HeaderBox = styled(Box)<{ status: string }>(({ theme, status }) => ({
  ...getStatusTintStyles(theme, status, 1),
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: 24,
  padding: theme.spacing(2),
}))

const LoadingBox = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  padding: theme.spacing(4),
}))

const StyledPaper = styled(Paper)<{ status?: string }>(({ theme, status }) => ({
  padding: theme.spacing(3),
  marginBottom: theme.spacing(2),
  backgroundColor: theme.palette.background.paper,
  ...(status ? getStatusTintStyles(theme, status, 'inherit') : {}),
}))

const StyledButton = styled(Button)(({ theme }) => ({
  backgroundColor: theme.palette.mode === 'dark' ? theme.palette.grey[800] : 'white',
  color: theme.palette.text.primary,
  '&:hover': {
    backgroundColor:
      theme.palette.mode === 'dark' ? theme.palette.grey[700] : theme.palette.grey[100],
  },
}))

const StyledDataGrid = styled(DataGrid)(({ theme }) => ({
  backgroundColor: theme.palette.background.paper,
  color: theme.palette.text.primary,
  '& .MuiDataGrid-main': {
    backgroundColor: theme.palette.background.paper,
  },
  '& .MuiDataGrid-columnHeaders': {
    backgroundColor: `${theme.palette.background.paper} !important`,
    color: theme.palette.text.primary,
    borderBottom: `1px solid ${theme.palette.divider}`,
  },
  '& .MuiDataGrid-columnHeader': {
    backgroundColor: `${theme.palette.background.paper} !important`,
    color: theme.palette.text.primary,
  },
  '& .MuiDataGrid-columnHeaderTitle': {
    color: theme.palette.text.primary,
    fontWeight: 600,
  },
  '& .MuiDataGrid-cell': {
    borderBottom: `1px solid ${theme.palette.divider}`,
    color: theme.palette.text.primary,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  '& .MuiDataGrid-row:hover': {
    backgroundColor: theme.palette.action.hover,
  },
  '& .MuiDataGrid-footerContainer': {
    backgroundColor: theme.palette.background.default,
    color: theme.palette.text.primary,
  },
}))

const SubComponentDescription = styled(Typography)<{
  hasLongDescription?: boolean
  hasTags?: boolean
}>(({ theme, hasLongDescription, hasTags }) => ({
  marginBottom: hasLongDescription || hasTags ? theme.spacing(2) : 0,
}))

const SubComponentLongDescription = styled(Typography)<{
  hasDocumentation?: boolean
  hasTags?: boolean
}>(({ theme, hasDocumentation, hasTags }) => ({
  color: theme.palette.text.secondary,
  whiteSpace: 'pre-wrap',
  lineHeight: 1.6,
  marginBottom: hasDocumentation || hasTags ? theme.spacing(2) : 0,
}))

const DocumentationButtonContainer = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(2),
}))

const TagsContainer = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(2),
}))

const SubComponentDetails = () => {
  const navigate = useNavigate()
  const { componentSlug, subComponentSlug } = useParams<{
    componentSlug: string
    subComponentSlug: string
  }>()
  const { isComponentAdmin } = useAuth()
  const { getTag } = useTags()
  const [outages, setOutages] = useState<Outage[]>([])
  const [error, setError] = useState<string | null>(null)
  const [createOutageModalOpen, setCreateOutageModalOpen] = useState(false)
  const [subComponentStatus, setSubComponentStatus] = useState<ComponentStatus | null>(null)
  const [subComponent, setSubComponent] = useState<SubComponent | null>(null)
  const [statusFilter, setStatusFilter] = useState<'all' | 'ongoing' | 'resolved'>('all')

  const componentName = componentSlug ? deslugify(componentSlug) : ''
  const subComponentName = subComponentSlug ? deslugify(subComponentSlug) : ''
  const isAdmin = isComponentAdmin(componentSlug || '')

  const validationError =
    !componentName || !subComponentName ? 'Missing component or subcomponent name' : null
  const [loading, setLoading] = useState(!!(componentName && subComponentName))

  const fetchData = useCallback(() => {
    if (!componentName || !subComponentName) {
      return
    }

    // Use setTimeout to defer state updates, then start async fetch
    setTimeout(() => {
      setLoading(true)
      setError(null)

      // Fetch outages, status, and component configuration in parallel
      Promise.all([
        fetch(getSubComponentOutagesEndpoint(componentName, subComponentName)),
        fetch(getSubComponentStatusEndpoint(componentName, subComponentName)),
        fetch(getComponentInfoEndpoint(componentName)),
      ])
        .then(([outagesResponse, statusResponse, componentResponse]) => {
          if (!outagesResponse.ok) {
            setError(`Failed to fetch outages: ${outagesResponse.statusText}`)
            return
          }
          if (!statusResponse.ok) {
            setError(`Failed to fetch status: ${statusResponse.statusText}`)
            return
          }
          if (!componentResponse.ok) {
            setError(`Failed to fetch component: ${componentResponse.statusText}`)
            return
          }
          return Promise.all([
            outagesResponse.json(),
            statusResponse.json(),
            componentResponse.json(),
          ])
        })
        .then((results) => {
          if (results) {
            const [outagesData, statusData, componentData] = results
            if (outagesData) {
              setOutages(outagesData)
              // Set default filter to 'ongoing' if there are any ongoing outages
              const hasOngoing = outagesData.some((outage: Outage) => !outage.end_time.Valid)
              if (hasOngoing) {
                setStatusFilter('ongoing')
              } else {
                setStatusFilter('all')
              }
            }
            if (statusData) {
              setSubComponentStatus(statusData)
            }
            if (componentData) {
              // Store the entire subcomponent configuration
              const foundSubComponent = componentData.sub_components.find(
                (sub: SubComponent) => sub.slug === subComponentSlug,
              )
              if (foundSubComponent) {
                setSubComponent(foundSubComponent)
              }
            }
          }
        })
        .catch(() => {
          setError('Failed to fetch data')
        })
        .finally(() => {
          setLoading(false)
        })
    }, 0)
  }, [componentName, subComponentName, subComponentSlug])

  useEffect(() => {
    if (!componentName || !subComponentName) {
      return
    }
    fetchData()
  }, [componentName, subComponentName, subComponentSlug, fetchData])

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString()
  }

  const getStatusText = (outage: Outage) => {
    if (outage.end_time.Valid) {
      return 'Resolved'
    }
    return 'Active'
  }

  const handleOutageAction = () => {
    fetchData()
  }

  const columns: GridColDef[] = [
    {
      field: 'status',
      headerName: 'Status',
      width: 80,
      renderCell: (params) => {
        const outage = params.row as Outage
        const status = getStatusText(outage)
        const isActive = status === 'Active'

        return (
          <Tooltip title={status} arrow>
            {isActive ? <ErrorIcon color="error" /> : <CheckCircle color="success" />}
          </Tooltip>
        )
      },
    },
    {
      field: 'severity',
      headerName: 'Severity',
      width: 120,
      renderCell: (params) => (
        <SeverityChip
          label={formatStatusSeverityText(params.value)}
          severity={params.value}
          size="small"
          variant="outlined"
        />
      ),
    },
    ...(subComponent?.requires_confirmation
      ? [
          {
            field: 'confirmation',
            headerName: 'Confirmation',
            width: 120,
            sortable: false,
            filterable: false,
            renderCell: (params: GridRenderCellParams) => {
              const outage = params.row as Outage
              const isConfirmed = outage.confirmed_at.Valid

              return (
                <Tooltip title={isConfirmed ? 'Confirmed' : 'Unconfirmed'} arrow>
                  {isConfirmed ? <CheckCircle color="success" /> : <Warning color="warning" />}
                </Tooltip>
              )
            },
          },
        ]
      : []),
    {
      field: 'description',
      headerName: 'Description',
      flex: 1,
      minWidth: 200,
      renderCell: (params) => (
        <Typography variant="body2" noWrap title={params.value || 'No description'}>
          {params.value || 'No description'}
        </Typography>
      ),
    },
    {
      field: 'start_time',
      headerName: 'Start Time',
      width: 120,
      renderCell: (params) => {
        const startDate = new Date(params.value)
        const now = new Date()
        const relative = relativeTime(startDate, now)
        return (
          <Typography variant="body2" title={formatDate(params.value)}>
            {relative}
          </Typography>
        )
      },
    },
    {
      field: 'end_time',
      headerName: 'End Time',
      width: 120,
      renderCell: (params) => {
        const outage = params.row as Outage
        if (outage.end_time.Valid) {
          const endDate = new Date(outage.end_time.Time)
          const now = new Date()
          const relative = relativeTime(endDate, now)
          return (
            <Typography variant="body2" title={formatDate(outage.end_time.Time)}>
              {relative}
            </Typography>
          )
        }
        return (
          <Typography variant="body2" color="error">
            Ongoing
          </Typography>
        )
      },
    },
    {
      field: 'details',
      headerName: 'Details',
      width: 100,
      sortable: false,
      filterable: false,
      renderCell: (params) => {
        const outage = params.row as Outage
        return <OutageDetailsButton outage={outage} />
      },
    },
    ...(isAdmin
      ? [
          {
            field: 'actions',
            headerName: 'Actions',
            width: 100,
            sortable: false,
            filterable: false,
            renderCell: (params: GridRenderCellParams) => {
              const outage = params.row as Outage
              return (
                <OutageActions outage={outage} onSuccess={handleOutageAction} onError={setError} />
              )
            },
          },
        ]
      : []),
  ]

  // Filter outages based on selected filter
  const filteredOutages = outages.filter((outage) => {
    if (statusFilter === 'ongoing') {
      return !outage.end_time.Valid
    }
    if (statusFilter === 'resolved') {
      return outage.end_time.Valid
    }
    return true // 'all'
  })

  // Sort outages: active first, then by start time descending
  const sortedOutages = [...filteredOutages].sort((a, b) => {
    const aActive = !a.end_time.Valid
    const bActive = !b.end_time.Valid

    if (aActive && !bActive) return -1
    if (!aActive && bActive) return 1

    return new Date(b.start_time).getTime() - new Date(a.start_time).getTime()
  })

  const handleFilterChange = (
    _event: React.MouseEvent<HTMLElement>,
    newFilter: 'all' | 'ongoing' | 'resolved' | null,
  ) => {
    if (newFilter !== null) {
      setStatusFilter(newFilter)
    }
  }

  if (!componentName || !subComponentName) {
    return (
      <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }} data-tour="subcomponent-detail">
        <Alert severity="error">Invalid component or subcomponent</Alert>
      </Container>
    )
  }

  return (
    <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }} data-tour="subcomponent-detail">
      <StyledPaper status={subComponentStatus?.status || 'Unknown'}>
        <HeaderBox
          status={subComponentStatus?.status || 'Unknown'}
          data-tour="subcomponent-detail-header"
        >
          <Box>
            <Typography variant="h4">
              {componentName} / {subComponentName} - Outages
            </Typography>
            {subComponentStatus?.last_ping_time && subComponent?.monitoring?.frequency && (
              <Typography variant="body2" sx={{ mt: 1, opacity: 0.8 }}>
                Last Checked:{' '}
                {relativeTime(new Date(subComponentStatus.last_ping_time), new Date())} · Expected
                Frequency: {subComponent.monitoring.frequency}
              </Typography>
            )}
          </Box>
          <Box sx={{ display: 'flex', gap: 2 }}>
            {isAdmin && (
              <Button
                variant="contained"
                color="error"
                startIcon={<ReportProblem />}
                onClick={() => setCreateOutageModalOpen(true)}
                data-tour="subcomponent-report-outage"
              >
                Report Outage
              </Button>
            )}
            <StyledButton
              variant="contained"
              onClick={() => navigate(`/${componentSlug}`)}
              data-tour="subcomponent-detail-component-link"
            >
              {componentName} Details
            </StyledButton>
          </Box>
        </HeaderBox>
      </StyledPaper>

      {(subComponent?.description ||
        subComponent?.long_description ||
        subComponent?.documentation_url ||
        (subComponent?.tags && subComponent.tags.length > 0)) && (
        <StyledPaper>
          {subComponent?.description && (
            <SubComponentDescription
              variant="body1"
              hasLongDescription={!!subComponent?.long_description}
              hasTags={!!(subComponent?.tags && subComponent.tags.length > 0)}
            >
              {subComponent.description}
            </SubComponentDescription>
          )}
          {subComponent?.tags && subComponent.tags.length > 0 && (
            <TagsContainer>
              {subComponent.tags.map((tag) => (
                <TagChip key={tag} tag={tag} size="small" color={getTag(tag)?.color} />
              ))}
            </TagsContainer>
          )}
          {subComponent?.long_description && (
            <SubComponentLongDescription
              variant="body2"
              hasDocumentation={!!subComponent?.documentation_url}
              hasTags={!!(subComponent?.tags && subComponent.tags.length > 0)}
            >
              {subComponent.long_description}
            </SubComponentLongDescription>
          )}
          {subComponent?.documentation_url && (
            <DocumentationButtonContainer>
              <Button
                variant="outlined"
                component="a"
                startIcon={<OpenInNew />}
                href={subComponent.documentation_url}
                target="_blank"
                rel="noopener noreferrer"
              >
                View Documentation
              </Button>
            </DocumentationButtonContainer>
          )}
        </StyledPaper>
      )}

      <StyledPaper>
        {loading && (
          <LoadingBox>
            <CircularProgress />
          </LoadingBox>
        )}

        {(validationError || error) && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {validationError || error}
          </Alert>
        )}

        {!loading && !validationError && !error && (
          <Box>
            <Box
              sx={{ display: 'flex', justifyContent: 'flex-end', mb: 2 }}
              data-tour="subcomponent-detail-filter"
            >
              <ToggleButtonGroup
                value={statusFilter}
                exclusive
                onChange={handleFilterChange}
                aria-label="outage status filter"
                size="small"
              >
                <ToggleButton value="all" aria-label="all outages">
                  All
                </ToggleButton>
                <ToggleButton value="ongoing" aria-label="ongoing outages">
                  Ongoing
                </ToggleButton>
                <ToggleButton value="resolved" aria-label="resolved outages">
                  Resolved
                </ToggleButton>
              </ToggleButtonGroup>
            </Box>
            <Box sx={{ height: 600, width: '100%' }} data-tour="subcomponent-detail-grid">
              <StyledDataGrid
                rows={sortedOutages}
                columns={columns}
                pageSizeOptions={[10, 25, 50, 100]}
                initialState={{
                  pagination: {
                    paginationModel: { pageSize: 25 },
                  },
                }}
                disableRowSelectionOnClick
                getRowId={(row) => row.ID}
              />
            </Box>
          </Box>
        )}
      </StyledPaper>

      <UpsertOutageModal
        open={createOutageModalOpen}
        onClose={() => setCreateOutageModalOpen(false)}
        onSuccess={handleOutageAction}
        componentName={componentName || ''}
        subComponentName={subComponentName || ''}
      />
    </Container>
  )
}

export default SubComponentDetails
