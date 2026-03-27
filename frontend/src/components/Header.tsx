import { Accessibility, Brightness4, Brightness7, HelpOutline, Insights } from '@mui/icons-material'
import { AppBar, Box, IconButton, styled, Toolbar, Tooltip } from '@mui/material'
import { useNavigate } from 'react-router-dom'

import Auth from './Auth'
import { TOUR_RESTART_EVENT, useHasTour } from './tour/AppTour'

interface HeaderProps {
  onToggleTheme: () => void
  isDarkMode: boolean
  onToggleAccessibility: () => void
  isAccessibilityMode: boolean
}

const DarkModeToggle = styled(IconButton)(({ theme }) => ({
  color: theme.palette.text.primary,
  backgroundColor: theme.palette.action.hover,
  '&:hover': {
    backgroundColor: theme.palette.action.selected,
  },
}))

const AccessibilityToggle = styled(IconButton)(({ theme }) => ({
  color: theme.palette.text.primary,
  backgroundColor: theme.palette.action.hover,
  '&:hover': {
    backgroundColor: theme.palette.action.selected,
  },
}))

const Header = ({
  onToggleTheme,
  isDarkMode,
  onToggleAccessibility,
  isAccessibilityMode,
}: HeaderProps) => {
  const navigate = useNavigate()
  const hasTour = useHasTour()

  const handleLogoClick = () => {
    navigate('/')
  }

  return (
    <AppBar
      position="sticky"
      sx={{
        backgroundColor: 'background.paper',
        boxShadow: 1,
        zIndex: 1000,
      }}
    >
      <Toolbar sx={{ justifyContent: 'space-between' }}>
        <Box
          component="img"
          src={isDarkMode ? '/logo-dark.svg' : '/logo.svg'}
          alt="Logo"
          onClick={handleLogoClick}
          sx={{
            height: 40,
            width: 'auto',
            maxWidth: 200,
            cursor: 'pointer',
            '&:hover': {
              opacity: 0.8,
            },
          }}
        />

        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Tooltip title={hasTour ? 'Page tour' : 'Page tour unavailable'}>
            <span style={{ display: 'inline-flex' }} data-tour="page-tour-button">
              <AccessibilityToggle
                disabled={!hasTour}
                onClick={() => window.dispatchEvent(new CustomEvent(TOUR_RESTART_EVENT))}
                aria-label="Page tour"
              >
                <HelpOutline />
              </AccessibilityToggle>
            </span>
          </Tooltip>
          <Tooltip
            title={isAccessibilityMode ? 'Disable accessibility mode' : 'Enable accessibility mode'}
          >
            <AccessibilityToggle
              onClick={onToggleAccessibility}
              aria-label="Toggle accessibility mode"
            >
              <Accessibility color={isAccessibilityMode ? 'primary' : 'inherit'} />
            </AccessibilityToggle>
          </Tooltip>
          <Tooltip title="SHIP Statistical Process Controls">
            <AccessibilityToggle
              onClick={() => navigate('/pages/spc-dashboard')}
              aria-label="SHIP Statistical Process Controls"
              data-tour="spc-dashboard-button"
            >
              <Insights />
            </AccessibilityToggle>
          </Tooltip>
          <Tooltip title={isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}>
            <DarkModeToggle onClick={onToggleTheme}>
              {isDarkMode ? <Brightness7 /> : <Brightness4 />}
            </DarkModeToggle>
          </Tooltip>
          <Auth />
        </Box>
      </Toolbar>
    </AppBar>
  )
}

export default Header
