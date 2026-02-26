import { Box, Button, Card, CardContent, styled, Typography } from '@mui/material'
import { useNavigate } from 'react-router-dom'

import type { Component, SubComponent } from '../../types'
import { formatStatusSeverityText } from '../../utils/helpers'
import { slugify } from '../../utils/slugify'
import { getStatusTintStyles } from '../../utils/styles'
import { StatusChip } from '../StatusColors'
import SubComponentCard from '../sub-component/SubComponentCard'

const ComponentWell = styled(Card)<{ status: string }>(({ theme, status }) => ({
  ...getStatusTintStyles(theme, status, 2),
  borderRadius: theme.spacing(2),
  transition: 'all 0.2s ease-in-out',
  '&:hover': {
    boxShadow: theme.shadows[6],
    transform: 'translateY(-2px)',
  },
}))

const SubComponentsGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
  gap: theme.spacing(2),
  marginTop: theme.spacing(2),
}))

const HeaderBox = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: theme.spacing(3),
  paddingBottom: theme.spacing(2),
  borderBottom: `1px solid ${theme.palette.divider}`,
}))

const FooterBox = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'flex-end',
  alignItems: 'center',
  marginTop: theme.spacing(3),
  paddingTop: theme.spacing(2),
  borderTop: `1px solid ${theme.palette.divider}`,
}))

const DetailsButton = styled(Button)(({ theme }) => ({
  minWidth: 'auto',
  padding: theme.spacing(1, 2),
  borderRadius: theme.spacing(1),
  textTransform: 'none',
  fontWeight: 500,
  backgroundColor:
    theme.palette.mode === 'dark' ? 'rgba(0, 0, 0, 0.7)' : 'rgba(255, 255, 255, 0.9)',
  color: theme.palette.text.primary,
  backdropFilter: 'blur(4px)',
  '&:hover': {
    backgroundColor:
      theme.palette.mode === 'dark' ? 'rgba(0, 0, 0, 0.9)' : 'rgba(255, 255, 255, 1)',
  },
}))

const ComponentTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1.5rem',
  color: theme.palette.text.primary,
}))

const DescriptionTypography = styled(Typography)(({ theme }) => ({
  marginBottom: theme.spacing(3),
  fontSize: '1rem',
  lineHeight: 1.6,
  color: theme.palette.text.secondary,
}))

interface ComponentWellProps {
  component: Component
}

const ComponentWellComponent = ({ component }: ComponentWellProps) => {
  const navigate = useNavigate()

  const handleDetailsClick = () => {
    navigate(`/${slugify(component.name)}`)
  }

  return (
    <ComponentWell status={component.status || 'Unknown'} data-tour="component-well">
      <CardContent>
        <HeaderBox>
          <ComponentTitle>{component.name}</ComponentTitle>
          <StatusChip
            label={formatStatusSeverityText(component.status || 'Unknown')}
            status={component.status || 'Unknown'}
            variant="filled"
          />
        </HeaderBox>

        <DescriptionTypography>{component.description}</DescriptionTypography>

        <SubComponentsGrid>
          {component.sub_components.map((subComponent: SubComponent) => (
            <SubComponentCard
              key={subComponent.name}
              subComponent={subComponent}
              componentName={component.name}
            />
          ))}
        </SubComponentsGrid>

        <FooterBox>
          <DetailsButton variant="outlined" onClick={handleDetailsClick} size="small">
            Details
          </DetailsButton>
        </FooterBox>
      </CardContent>
    </ComponentWell>
  )
}

export default ComponentWellComponent
