import CssBaseline from '@mui/material/CssBaseline'
import { ThemeProvider } from '@mui/material/styles'
import { StylesProvider } from '@mui/styles'
import { useEffect, useMemo, useState } from 'react'
import { Route, BrowserRouter as Router, Routes, useLocation } from 'react-router-dom'

import ComponentDetailsPage from './components/component/ComponentDetailsPage'
import ComponentStatusList from './components/ComponentStatusList'
import Header from './components/Header'
import OutageDetailsPage from './components/outage/OutageDetailsPage'
import SubComponentDetails from './components/sub-component/SubComponentDetails'
import TagPage from './components/tags/TagPage'
import AppTour from './components/tour/AppTour'
import { AuthProvider } from './contexts/AuthContext'
import { TagsProvider } from './contexts/TagsContext'
import { darkAccessibilityTheme, darkTheme, lightAccessibilityTheme, lightTheme } from './themes'
import { getProtectedDomain, getPublicDomain } from './utils/endpoints'

// This component is used to redirect the user to the public domain if they are on the protected domain
// This is necessary because the oauth proxy will redirect the user to the protected domain after authentication
// and we need to redirect them back to the public domain to avoid a redirect loop
function RedirectIfProtected() {
  const location = useLocation()

  useEffect(() => {
    const currentHost = window.location.hostname
    const protectedDomain = getProtectedDomain()
      .replace(/^https?:\/\//, '')
      .split('/')[0]
    const publicDomain = getPublicDomain()
      .replace(/^https?:\/\//, '')
      .split('/')[0]

    if (currentHost === protectedDomain) {
      const publicUrl = `${window.location.protocol}//${publicDomain}${location.pathname}${location.search}${location.hash}`
      console.log(`redirecting to: ${publicUrl}`)
      window.location.replace(publicUrl)
    }
  }, [location])

  return null
}

function App() {
  const [isDarkMode, setIsDarkMode] = useState(() => {
    const saved = localStorage.getItem('theme')
    if (saved) return saved === 'dark'
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  const [isAccessibilityMode, setIsAccessibilityMode] = useState(() => {
    const saved = localStorage.getItem('accessibilityMode')
    return saved === 'true'
  })

  const theme = useMemo(() => {
    if (isAccessibilityMode) {
      return isDarkMode ? darkAccessibilityTheme : lightAccessibilityTheme
    }
    return isDarkMode ? darkTheme : lightTheme
  }, [isDarkMode, isAccessibilityMode])

  const toggleTheme = () => {
    const newMode = !isDarkMode
    setIsDarkMode(newMode)
    localStorage.setItem('theme', newMode ? 'dark' : 'light')
    window.dispatchEvent(new CustomEvent('themeChanged'))
  }

  const toggleAccessibilityMode = () => {
    const newMode = !isAccessibilityMode
    setIsAccessibilityMode(newMode)
    localStorage.setItem('accessibilityMode', newMode.toString())
    window.dispatchEvent(new CustomEvent('themeChanged'))
  }

  return (
    <StylesProvider injectFirst>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <AuthProvider>
          <TagsProvider>
            <Router>
              <RedirectIfProtected />
              <Header
                onToggleTheme={toggleTheme}
                isDarkMode={isDarkMode}
                onToggleAccessibility={toggleAccessibilityMode}
                isAccessibilityMode={isAccessibilityMode}
              />
              <Routes>
                <Route path="/" element={<ComponentStatusList />} />
                <Route path="/tags/:tag" element={<TagPage />} />
                <Route path="/:componentSlug" element={<ComponentDetailsPage />} />
                <Route path="/:componentSlug/:subComponentSlug" element={<SubComponentDetails />} />
                <Route
                  path="/:componentSlug/:subComponentSlug/outages/:outageId"
                  element={<OutageDetailsPage />}
                />
              </Routes>
              <AppTour />
            </Router>
          </TagsProvider>
        </AuthProvider>
      </ThemeProvider>
    </StylesProvider>
  )
}

export default App
