import { Box, Card, CardContent, Tooltip, Typography, styled } from '@mui/material'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useTags } from '../../contexts/TagsContext'
import type { SubComponent } from '../../types'
import { getSubComponentStatusEndpoint } from '../../utils/endpoints'
import { formatStatusSeverityText } from '../../utils/helpers'
import { deslugify, slugify } from '../../utils/slugify'
import { getStatusTintStyles } from '../../utils/styles'
import { StatusChip } from '../StatusColors'
import TagChip from '../tags/TagChip'

const SubComponentCard = styled(Card)<{ status: string }>(({ theme, status }) => ({
  ...getStatusTintStyles(theme, status, 1.5),
  borderRadius: theme.spacing(1.5),
  cursor: 'pointer',
  transition: 'all 0.2s ease-in-out',
  minHeight: '160px',
  display: 'flex',
  flexDirection: 'column',
  '&:hover': {
    boxShadow: theme.shadows[4],
    transform: 'translateY(-1px)',
    '& .MuiChip-root': {
      opacity: 0.9,
    },
  },
}))

const StyledCardContent = styled(CardContent)(({ theme }) => ({
  padding: theme.spacing(2.5),
  flex: 1,
  display: 'flex',
  flexDirection: 'column',
  '&:last-child': {
    paddingBottom: theme.spacing(2.5),
  },
}))

const CardHeader = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  marginBottom: theme.spacing(1.5),
}))

const SubComponentTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1rem',
  color: theme.palette.text.primary,
  flex: 1,
  marginRight: theme.spacing(1),
}))

const SubComponentDescription = styled(Typography)(({ theme }) => ({
  fontSize: '0.875rem',
  color: theme.palette.text.secondary,
  lineHeight: 1.5,
  flex: 1,
  marginBottom: theme.spacing(1),
}))

const StatusChipBox = styled(Box)(() => ({
  flexShrink: 0,
}))

const CardFooter = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  marginTop: theme.spacing(1.5),
  paddingTop: theme.spacing(1.5),
  borderTop: `1px solid ${theme.palette.divider}`,
}))

const TagsContainer = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(0.5),
  flex: 1,
}))

interface SubComponentCardProps {
  subComponent: SubComponent
  componentName: string
}

const SubComponentCardComponent = ({ subComponent, componentName }: SubComponentCardProps) => {
  const navigate = useNavigate()
  const { getTag } = useTags()
  const [subComponentWithStatus, setSubComponentWithStatus] = useState<SubComponent>(subComponent)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch(getSubComponentStatusEndpoint(componentName, subComponent.name))
      .then((res) => res.json().catch(() => ({ status: 'Unknown', active_outages: [] })))
      .then((subStatus) => {
        setSubComponentWithStatus({
          ...subComponent,
          status: subStatus.status,
          active_outages: subStatus.active_outages,
        })
      })
      .finally(() => {
        setLoading(false)
      })
  }, [componentName, subComponent])

  const handleClick = () => {
    const status = subComponentWithStatus.status || 'Unknown'
    const activeOutages = subComponentWithStatus.active_outages || []
    const isHealthy = status === 'Healthy' || activeOutages.length === 0

    if (isHealthy || activeOutages.length > 1) {
      navigate(`/${slugify(componentName)}/${slugify(subComponentWithStatus.name)}`)
    } else if (activeOutages.length === 1) {
      navigate(
        `/${slugify(componentName)}/${slugify(subComponentWithStatus.name)}/outages/${activeOutages[0].ID}`,
      )
    }
  }

  const cardContent = (
    <SubComponentCard
      status={subComponentWithStatus.status || 'Unknown'}
      onClick={handleClick}
      data-tour="subcomponent-card"
    >
      <StyledCardContent>
        <CardHeader>
          <SubComponentTitle>{deslugify(subComponent.name)}</SubComponentTitle>
          <StatusChipBox>
            <StatusChip
              label={
                loading
                  ? 'Loading...'
                  : formatStatusSeverityText(subComponentWithStatus.status || 'Unknown')
              }
              status={subComponentWithStatus.status || 'Unknown'}
              size="small"
              variant="filled"
            />
          </StatusChipBox>
        </CardHeader>
        <SubComponentDescription>{subComponent.description}</SubComponentDescription>
        <CardFooter data-tour="subcomponent-tags">
          <TagsContainer>
            {subComponent.tags?.map((tag) => (
              <TagChip key={tag} tag={tag} size="small" color={getTag(tag)?.color} />
            ))}
          </TagsContainer>
        </CardFooter>
      </StyledCardContent>
    </SubComponentCard>
  )

  if (subComponent.long_description) {
    return (
      <Tooltip
        title={subComponent.long_description}
        arrow
        placement="top"
        enterDelay={300}
        leaveDelay={100}
        slotProps={{
          tooltip: {
            sx: (theme) =>
              theme.palette.mode === 'light'
                ? {
                    backgroundColor: '#ffffff',
                    color: '#000000',
                    border: `1px solid ${theme.palette.grey[700]}`,
                  }
                : { border: '1px solid #ffffff' },
          },
        }}
      >
        {cardContent}
      </Tooltip>
    )
  }

  return cardContent
}

export default SubComponentCardComponent
