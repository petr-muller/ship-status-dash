import {
  AccessTime,
  ArrowBack,
  Assignment,
  BugReport,
  Forum,
  Info,
  Person,
  Settings,
} from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Paper,
  Typography,
  styled,
} from '@mui/material'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import type { Outage } from '../../types'
import { getOutageEndpoint } from '../../utils/endpoints'
import {
  formatDuration,
  formatStatusSeverityText,
  getStatusChipColor,
  relativeTime,
} from '../../utils/helpers'
import { deslugify, slugify } from '../../utils/slugify'
import { getStatusTintStyles } from '../../utils/styles'

import OutageActions from './actions/OutageActions'
import Field, { FieldBox, FieldLabel } from './OutageDetailsField'
import Section from './OutageDetailsSection'

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const HeaderPaper = styled(Paper)<{
  severity: string
  resolved: boolean
  unconfirmed: boolean
}>(({ theme, severity, resolved, unconfirmed }) => {
  let severityStatus: string
  if (resolved) {
    severityStatus = 'Healthy'
  } else if (unconfirmed) {
    severityStatus = 'Suspected'
  } else {
    severityStatus = severity
  }

  return {
    ...getStatusTintStyles(theme, severityStatus, 2),
    padding: theme.spacing(4),
    marginBottom: theme.spacing(3),
    borderRadius: theme.spacing(2),
    boxShadow: theme.shadows[4],
  }
})

const BackButton = styled(Button)(({ theme }) => ({
  marginBottom: theme.spacing(3),
}))

const PageTitle = styled(Typography)(() => ({
  fontWeight: 600,
  marginBottom: 8,
}))

const ChipSpacer = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(0.5),
}))

const SeverityChip = styled(Chip)<{ severity: string }>(({ theme, severity }) => {
  const colorValue = getStatusChipColor(theme, severity)
  return {
    backgroundColor: colorValue,
    color: theme.palette.getContrastText(colorValue),
  }
})

const HeaderChip = styled(SeverityChip)(() => ({
  fontSize: '0.95rem',
  fontWeight: 600,
  height: 32,
}))

const ResolvedChip = styled(Chip)(() => ({
  fontSize: '0.95rem',
  fontWeight: 600,
  height: 32,
}))

const GridContainer = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: '1fr',
  gap: theme.spacing(3),
  [theme.breakpoints.up('md')]: {
    gridTemplateColumns: 'repeat(2, 1fr)',
  },
}))

const FullWidthGridItem = styled(Box)(({ theme }) => ({
  gridColumn: '1',
  [theme.breakpoints.up('md')]: {
    gridColumn: '1 / -1',
  },
}))

const SystemGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: '1fr',
  gap: theme.spacing(2),
  [theme.breakpoints.up('sm')]: {
    gridTemplateColumns: 'repeat(3, 1fr)',
  },
}))

const ConfirmationChipContainer = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(0.5),
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  flexWrap: 'wrap',
}))

const HeaderContent = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  flexWrap: 'wrap',
  gap: theme.spacing(2),
}))

const HeaderTitleBox = styled(Box)(() => ({}))

const ErrorAlert = styled(Alert)(({ theme }) => ({
  marginBottom: theme.spacing(2),
}))

const LoadingContainer = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '400px',
}))

const TopActionsContainer = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'flex-end',
  alignItems: 'center',
  '& button': {
    border: `1px solid ${theme.palette.divider}`,
  },
}))

const ReasonCard = styled(Paper)(({ theme }) => ({
  padding: theme.spacing(2),
  marginBottom: theme.spacing(2),
  borderRadius: theme.spacing(1),
  border: `1px solid ${theme.palette.divider}`,
  '&:last-child': {
    marginBottom: 0,
  },
}))

const ReasonTypeChip = styled(Chip)(({ theme }) => ({
  marginBottom: theme.spacing(1),
  fontWeight: 600,
}))

const ReasonContentBox = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(1),
  '& > *:not(:last-child)': {
    marginBottom: theme.spacing(1),
  },
}))

const ReasonLabel = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  color: theme.palette.text.secondary,
  fontSize: '0.875rem',
  marginBottom: theme.spacing(0.5),
}))

const ReasonValue = styled(Typography)(() => ({
  fontFamily: 'monospace',
  fontSize: '0.875rem',
  wordBreak: 'break-word',
}))

const ResultsContainer = styled(Box)(({ theme }) => ({
  maxHeight: '300px',
  overflowY: 'auto',
  padding: theme.spacing(1),
  backgroundColor: theme.palette.mode === 'dark' ? theme.palette.grey[900] : theme.palette.grey[50],
  borderRadius: theme.spacing(0.5),
  border: `1px solid ${theme.palette.divider}`,
}))

const ResultItem = styled(Typography)(({ theme }) => ({
  fontFamily: 'monospace',
  fontSize: '0.8125rem',
  padding: theme.spacing(0.5),
  marginBottom: theme.spacing(0.25),
  wordBreak: 'break-word',
  '&:last-child': {
    marginBottom: 0,
  },
}))

const OutageDetailsPage = () => {
  const navigate = useNavigate()
  const { componentSlug, subComponentSlug, outageId } = useParams<{
    componentSlug: string
    subComponentSlug: string
    outageId: string
  }>()
  const componentName = componentSlug ? deslugify(componentSlug) : ''
  const subComponentName = subComponentSlug ? deslugify(subComponentSlug) : ''
  const [outage, setOutage] = useState<Outage | null>(null)
  const [error, setError] = useState<string | null>(null)

  const validationError =
    !componentName || !subComponentName || !outageId
      ? 'Missing component, subcomponent, or outage ID'
      : null
  const [loading, setLoading] = useState(!!(componentName && subComponentName && outageId))

  const fetchOutage = useCallback(() => {
    if (!componentName || !subComponentName || !outageId) {
      return
    }

    const fetchPromise = fetch(
      getOutageEndpoint(componentName, subComponentName, parseInt(outageId, 10)),
    )

    fetchPromise.then(() => {
      setLoading(true)
      setError(null)
    })

    fetchPromise
      .then((outageResponse) => {
        if (!outageResponse.ok) {
          // If 404, outage was deleted, navigate back
          if (outageResponse.status === 404) {
            if (componentName && subComponentName) {
              navigate(`/${slugify(componentName)}/${slugify(subComponentName)}`)
            } else {
              navigate('/')
            }
            return
          }
          throw new Error(`Failed to fetch outage: ${outageResponse.statusText}`)
        }
        return outageResponse.json()
      })
      .then((outageData) => {
        if (outageData) {
          setOutage(outageData)
        }
      })
      .catch((err) => {
        setError(err.message || 'Failed to fetch data')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [componentName, subComponentName, outageId, navigate])

  useEffect(() => {
    if (!componentName || !subComponentName || !outageId) {
      return
    }

    fetchOutage()
  }, [componentName, subComponentName, outageId, fetchOutage])

  const formatDateTime = (dateString: string) => {
    const date = new Date(dateString)
    return `${date.toLocaleString()} (${relativeTime(date, new Date())})`
  }

  const formatNullableDateTime = (nullableTime: { Time: string; Valid: boolean }) => {
    if (!nullableTime.Valid) {
      return 'Not set'
    }
    return formatDateTime(nullableTime.Time)
  }

  const handleBack = () => {
    if (componentSlug && subComponentSlug) {
      navigate(`/${componentSlug}/${subComponentSlug}`)
    } else {
      navigate('/')
    }
  }

  const isResolved = () => {
    if (!outage?.end_time.Valid) {
      return false
    }
    const endTime = new Date(outage.end_time.Time)
    const now = new Date()
    return endTime < now
  }

  const getBackButtonLabel = () => {
    if (outage) {
      return `${deslugify(outage.component_name)} / ${deslugify(outage.sub_component_name)} Outages`
    }
    return 'Go Back'
  }

  const formatResults = (results: string): string[] => {
    if (!results) return ['']
    const items = results
      .split(',')
      .map((item) => item.trim())
      .filter((item) => item.length > 0)
    return items.length > 0 ? items : [results]
  }

  if (loading) {
    return (
      <StyledContainer maxWidth="lg">
        <LoadingContainer>
          <CircularProgress />
        </LoadingContainer>
      </StyledContainer>
    )
  }

  if (validationError || error || !outage) {
    return (
      <StyledContainer maxWidth="lg">
        <ErrorAlert severity="error">{validationError || error || 'Outage not found'}</ErrorAlert>
        <BackButton onClick={handleBack} variant="contained" startIcon={<ArrowBack />}>
          {getBackButtonLabel()}
        </BackButton>
      </StyledContainer>
    )
  }

  return (
    <StyledContainer maxWidth="lg">
      <Box display="flex" justifyContent="space-between" alignItems="flex-start" marginBottom={3}>
        <BackButton onClick={handleBack} variant="outlined" startIcon={<ArrowBack />}>
          {getBackButtonLabel()}
        </BackButton>
        <TopActionsContainer data-tour="outage-detail-actions">
          {outage && <OutageActions outage={outage} onSuccess={fetchOutage} onError={setError} />}
        </TopActionsContainer>
      </Box>

      <HeaderPaper
        severity={outage.severity}
        resolved={isResolved()}
        unconfirmed={!outage.confirmed_at.Valid}
        elevation={2}
        data-tour="outage-detail-header"
      >
        <HeaderContent>
          <HeaderTitleBox>
            <PageTitle variant="h4">Outage Details</PageTitle>
            <Typography variant="body1" color="text.secondary">
              {deslugify(outage.component_name)} / {deslugify(outage.sub_component_name)}
            </Typography>
          </HeaderTitleBox>
          {isResolved() ? (
            <ResolvedChip label="Resolved" color="success" size="medium" />
          ) : (
            <HeaderChip
              label={formatStatusSeverityText(outage.severity)}
              severity={outage.severity}
              size="medium"
            />
          )}
        </HeaderContent>
      </HeaderPaper>

      <GridContainer>
        <Section icon={<Info />} title="Basic Information">
          <Field label="Component" value={deslugify(outage.component_name)} />
          <Field label="Sub-Component" value={deslugify(outage.sub_component_name)} />
          <FieldBox>
            <FieldLabel variant="caption" color="text.secondary">
              Severity
            </FieldLabel>
            <ChipSpacer>
              <SeverityChip
                label={formatStatusSeverityText(outage.severity)}
                severity={outage.severity}
                size="small"
              />
            </ChipSpacer>
          </FieldBox>
          {outage.description && <Field label="Description" value={outage.description} />}
        </Section>

        <Section icon={<AccessTime />} title="Timing Information">
          <Field label="Start Time" value={formatDateTime(outage.start_time)} />
          <Field label="End Time" value={formatNullableDateTime(outage.end_time)} />
          <Field
            label="Duration"
            value={formatDuration(outage.start_time, outage.end_time)}
            valueVariant="primary"
          />
        </Section>

        {outage.reasons && outage.reasons.length > 0 && (
          <FullWidthGridItem>
            <Section icon={<BugReport />} title="Automated Monitoring Failures">
              {outage.reasons.map((reason) => (
                <ReasonCard key={reason.ID} elevation={0}>
                  <ReasonTypeChip
                    label={reason.type}
                    color="primary"
                    size="small"
                    variant="outlined"
                  />
                  <ReasonContentBox>
                    <Box>
                      <ReasonLabel variant="caption">Health Check</ReasonLabel>
                      <ReasonValue variant="body2">{reason.check}</ReasonValue>
                    </Box>
                    <Box>
                      <ReasonLabel variant="caption">Failure Results</ReasonLabel>
                      <ResultsContainer>
                        {formatResults(reason.results).map((item, index) => (
                          <ResultItem key={index} variant="body2">
                            {item}
                          </ResultItem>
                        ))}
                      </ResultsContainer>
                    </Box>
                  </ReasonContentBox>
                </ReasonCard>
              ))}
            </Section>
          </FullWidthGridItem>
        )}

        <Section icon={<Person />} title="User Information">
          <Field label="Created By" value={outage.created_by} />
          {outage.resolved_by && <Field label="Resolved By" value={outage.resolved_by} />}
          {outage.confirmed_by && <Field label="Confirmed By" value={outage.confirmed_by} />}
        </Section>

        <Section icon={<Assignment />} title="Additional Information">
          <Field label="Discovered From" value={outage.discovered_from} />
          <FieldBox>
            <FieldLabel variant="caption" color="text.secondary">
              Confirmed
            </FieldLabel>
            <ConfirmationChipContainer>
              <Chip
                label={outage.confirmed_at.Valid ? 'Yes' : 'No'}
                color={outage.confirmed_at.Valid ? 'success' : 'default'}
                size="small"
              />
              {outage.confirmed_at.Valid && (
                <Typography variant="body2" color="text.secondary">
                  {formatDateTime(outage.confirmed_at.Time)}
                </Typography>
              )}
            </ConfirmationChipContainer>
          </FieldBox>
          {outage.triage_notes && (
            <Field label="Triage Notes" value={outage.triage_notes} valueVariant="pre-wrap" />
          )}
        </Section>

        {outage.slack_threads && outage.slack_threads.length > 0 && (
          <Section icon={<Forum />} title="Automated Slack Reporting">
            {outage.slack_threads.map((thread) => (
              <FieldBox key={thread.channel}>
                <FieldLabel variant="caption" color="text.secondary">
                  {thread.channel}
                </FieldLabel>
                <Box>
                  <Button
                    variant="outlined"
                    size="small"
                    href={thread.thread_url}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    View Thread
                  </Button>
                </Box>
              </FieldBox>
            ))}
          </Section>
        )}

        <FullWidthGridItem>
          <Section icon={<Settings />} title="System Information">
            <SystemGrid>
              <Field label="Outage ID" value={outage.ID} valueVariant="monospace" />
              <Field label="Created At" value={formatDateTime(outage.CreatedAt)} />
              <Field label="Updated At" value={formatDateTime(outage.UpdatedAt)} />
            </SystemGrid>
          </Section>
        </FullWidthGridItem>
      </GridContainer>
    </StyledContainer>
  )
}

export default OutageDetailsPage
