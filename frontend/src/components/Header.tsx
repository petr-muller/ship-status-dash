import { Accessibility, Brightness4, Brightness7, HelpOutline, Insights } from '@mui/icons-material'
import { AppBar, Box, IconButton, styled, Toolbar, Tooltip } from '@mui/material'
import { useNavigate } from 'react-router-dom'

import { EXTERNAL_PAGES_PATH_PREFIX, externalPages } from '../constants/externalPages'
import Auth from './Auth'
import { TOUR_RESTART_EVENT, useHasTour } from './tour/AppTour'

interface HeaderProps {
  onToggleTheme: () => void
  isDarkMode: boolean
  onToggleAccessibility: () => void
  isAccessibilityMode: boolean
}

const HeaderIconButton = styled(IconButton)(({ theme }) => ({
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
  const spcPage = externalPages.find((p) => p.slug === 'spc-dashboard')

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
              <HeaderIconButton
                disabled={!hasTour}
                onClick={() => window.dispatchEvent(new CustomEvent(TOUR_RESTART_EVENT))}
                aria-label="Page tour"
              >
                <HelpOutline />
              </HeaderIconButton>
            </span>
          </Tooltip>
          <Tooltip
            title={isAccessibilityMode ? 'Disable accessibility mode' : 'Enable accessibility mode'}
          >
            <HeaderIconButton
              onClick={onToggleAccessibility}
              aria-label="Toggle accessibility mode"
            >
              <Accessibility color={isAccessibilityMode ? 'primary' : 'inherit'} />
            </HeaderIconButton>
          </Tooltip>
          {spcPage && (
            <Tooltip title={spcPage.description || spcPage.label}>
              <HeaderIconButton
                onClick={() => navigate(`${EXTERNAL_PAGES_PATH_PREFIX}/${spcPage.slug}`)}
                aria-label={spcPage.description || spcPage.label}
                data-tour="spc-dashboard-button"
              >
                <Insights />
              </HeaderIconButton>
            </Tooltip>
          )}
          <Tooltip title={isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}>
            <HeaderIconButton onClick={onToggleTheme}>
              {isDarkMode ? <Brightness7 /> : <Brightness4 />}
            </HeaderIconButton>
          </Tooltip>
          <Auth />
        </Box>
      </Toolbar>
    </AppBar>
  )
}

export default Header
