import { Alert, Box, CircularProgress, Container, styled, Typography } from '@mui/material'
import { useEffect, useState } from 'react'

import type { Component } from '../types'
import { getComponentsEndpoint, getOverallStatusEndpoint } from '../utils/endpoints'
import { slugify } from '../utils/slugify'

import ComponentWell from './component/ComponentWell'

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
}))

const LoadingBox = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '200px',
}))

const TitleSection = styled(Box)(({ theme }) => ({
  padding: theme.spacing(1, 0),
  marginBottom: theme.spacing(4),
  textAlign: 'center',
  borderBottom: `2px solid ${theme.palette.divider}`,
  backgroundColor: theme.palette.background.default,
}))

const TitleContainer = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  marginBottom: theme.spacing(1),
  gap: theme.spacing(2),
}))

const Logo = styled('img')(({ theme }) => ({
  height: '120px',
  width: 'auto',
  [theme.breakpoints.down('sm')]: {
    height: '80px',
  },
}))

const Subtitle = styled(Typography)(({ theme }) => ({
  fontSize: '1rem',
  color: theme.palette.text.secondary,
  fontWeight: 400,
}))

const ComponentsGrid = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(3),
}))

const ComponentStatusList = () => {
  const [components, setComponents] = useState<Component[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isDarkMode, setIsDarkMode] = useState(() => {
    const saved = localStorage.getItem('theme')
    return saved === 'dark'
  })

  useEffect(() => {
    // Listen for theme changes
    const handleThemeChange = () => {
      const saved = localStorage.getItem('theme')
      setIsDarkMode(saved === 'dark')
    }

    window.addEventListener('storage', handleThemeChange)
    window.addEventListener('themeChanged', handleThemeChange)
    return () => {
      window.removeEventListener('storage', handleThemeChange)
      window.removeEventListener('themeChanged', handleThemeChange)
    }
  }, [])

  useEffect(() => {
    // Fetch components configuration and their statuses
    Promise.all([
      fetch(getComponentsEndpoint()).then((res) => res.json()),
      fetch(getOverallStatusEndpoint()).then((res) => res.json()),
    ])
      .then(([componentsData, statusesData]) => {
        // Create a map of component statuses for quick lookup
        const statusMap = new Map<string, string>()
        statusesData.forEach((status: { component_name: string; status: string }) => {
          statusMap.set(status.component_name, status.status)
        })

        // Combine components with their statuses
        const componentsWithStatuses = componentsData.map((component: Component) => ({
          ...component,
          status: statusMap.get(component.name) || 'Unknown',
        }))

        return componentsWithStatuses
      })
      .then((data) => {
        setComponents(data)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to fetch components')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [])

  if (loading) {
    return (
      <StyledContainer maxWidth="lg">
        <LoadingBox>
          <CircularProgress />
        </LoadingBox>
      </StyledContainer>
    )
  }

  if (error) {
    return (
      <StyledContainer maxWidth="lg">
        <Alert severity="error">{error}</Alert>
      </StyledContainer>
    )
  }

  return (
    <StyledContainer maxWidth="lg">
      <TitleSection>
        <TitleContainer>
          <Logo src={isDarkMode ? '/logo-dark.svg' : '/logo.svg'} alt="SHIP Logo" />
        </TitleContainer>
        <Subtitle>Real-time monitoring of system components and availability</Subtitle>
      </TitleSection>

      <ComponentsGrid data-tour="component-list">
        {components.map((component) => (
          <ComponentWell key={slugify(component.name)} component={component} />
        ))}
      </ComponentsGrid>
    </StyledContainer>
  )
}

export default ComponentStatusList
