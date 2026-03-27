import { driver, type DriveStep } from 'driver.js'
import 'driver.js/dist/driver.css'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'

type TourStep = DriveStep & { waitForTarget?: boolean }

const TOUR_SEEN_ROUTE_TYPES_KEY = 'shipStatusTourSeenRouteTypes'
export const TOUR_RESTART_EVENT = 'shipStatusTourRestart'

const ROUTE_TYPES_WITH_TOURS = ['home', 'subcomponent-detail', 'outage-detail', 'external-page'] as const
type TourRouteType = (typeof ROUTE_TYPES_WITH_TOURS)[number]

function getRouteType(pathname: string): TourRouteType | null {
  if (pathname === '/') return 'home'
  if (pathname.startsWith('/pages/')) return 'external-page'
  if (pathname.includes('/outages/')) return 'outage-detail'
  const segments = pathname.split('/').filter(Boolean)
  if (segments.length === 2 && !pathname.startsWith('/tags')) return 'subcomponent-detail'
  return null
}

function getSeenRouteTypes(): TourRouteType[] {
  if (typeof window === 'undefined') return []
  try {
    const raw = localStorage.getItem(TOUR_SEEN_ROUTE_TYPES_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as unknown
    return Array.isArray(parsed) ? parsed.filter((t) => ROUTE_TYPES_WITH_TOURS.includes(t)) : []
  } catch {
    return []
  }
}

function markRouteTypeSeen(routeType: TourRouteType) {
  if (typeof window === 'undefined') return
  const seen = getSeenRouteTypes()
  if (seen.includes(routeType)) return
  seen.push(routeType)
  try {
    localStorage.setItem(TOUR_SEEN_ROUTE_TYPES_KEY, JSON.stringify(seen))
  } catch {
    // Ignore storage failures, worst case we show the tour again
  }
}

function handleDestroyed(routeType: TourRouteType | null) {
  if (routeType) markRouteTypeSeen(routeType)
}

function stepsWithExistingTargets(steps: DriveStep[]): DriveStep[] {
  if (typeof document === 'undefined') return steps
  const hasOutageActionsStep = steps.some((s) =>
    String(s.element).includes('outage-detail-actions'),
  )
  if (hasOutageActionsStep) {
    const headerEl = document.querySelector('[data-tour="outage-detail-header"]')
    const actionsEl = document.querySelector('[data-tour="outage-detail-actions"]')
    const hasActionsButton = actionsEl?.querySelector('button')
    if (!headerEl) return []
    if (!hasActionsButton) return steps.slice(0, 1)
    // Return all steps without filtering: the actions container is always present but only has a
    // button when the user is admin; the update/resolve targets live in the menu and only exist
    // after we open it during the tour.
    return steps
  }
  return steps.filter((s) => document.querySelector(String(s.element)))
}

function openOutageActionsMenuForTour() {
  const btn = document.querySelector('[data-tour="outage-detail-actions"] button')
  if (btn instanceof HTMLElement) btn.click()
}

function buildDriverConfig(steps: DriveStep[], initialRouteType: TourRouteType | null) {
  const actionsStepIndex = steps.findIndex((s) =>
    String(s.element).includes('outage-detail-actions'),
  )
  const isOutageMenuTour = actionsStepIndex >= 0 && steps[actionsStepIndex]?.popover
  const stepsToUse = isOutageMenuTour
    ? steps.map((s, i) =>
        i === actionsStepIndex
          ? {
              ...s,
              popover: {
                ...s.popover,
                onNextClick: (
                  _el: Element | undefined,
                  _step: DriveStep,
                  opts: {
                    driver: ReturnType<typeof driver>
                    state?: { activeIndex?: number }
                  },
                ) => {
                  openOutageActionsMenuForTour()
                  setTimeout(() => opts.driver.moveNext(), 200)
                },
              },
            }
          : s,
      )
    : steps
  return {
    steps: stepsToUse,
    showProgress: true,
    showButtons: ['next', 'previous', 'close'] as ('next' | 'previous' | 'close')[],
    onDestroyed: () => handleDestroyed(initialRouteType),
    onCloseClick: handleCloseClick,
  }
}

function runTourForCurrentPage() {
  const pathname = typeof window !== 'undefined' ? window.location.pathname : ''
  const steps = stepsWithExistingTargets(getStepsForRoute(pathname))
  if (steps.length === 0) return
  const initialRouteType = getRouteType(pathname)
  const driverObj = driver(buildDriverConfig(steps, initialRouteType))
  driverObj.drive()
}

function getStepsForRoute(pathname: string): TourStep[] {
  const segments = pathname.split('/').filter(Boolean)

  if (pathname === '/') {
    return [
      {
        element: '[data-tour="home-heading"]',
        popover: {
          title: 'Welcome',
          description:
            'This short tour will point out key areas on this page. Tours are also available on other pages throughout the application—look for the Page tour button when you want a guided overview.',
          side: 'bottom',
          align: 'center',
        },
      },
      {
        element: '[data-tour="component-well"]',
        popover: {
          title: 'Component',
          description:
            'Each well is a component that includes its overall status. Use the Details button to open the full component page and view more information.',
          side: 'bottom',
          align: 'center',
        },
      },
      {
        element: '[data-tour="component-well"] [data-tour="subcomponent-card"]',
        popover: {
          title: 'Sub-component',
          description:
            'Sub-components cards show their own status. If one has an active outage, clicking goes to that outage’s details; otherwise you’ll see the sub-component page with historical outage information.',
          side: 'top',
          align: 'center',
        },
      },
      {
        element: '[data-tour="component-well"] [data-tour="subcomponent-tags"]',
        popover: {
          title: 'Tags',
          description:
            'Tags categorize sub-components. Click a tag to see other sub-components with that tag.',
          side: 'top',
          align: 'center',
        },
      },
      {
        element: '[data-tour="login-button"]',
        popover: {
          title: 'Login',
          description:
            'Login is required for mutating actions, such as reporting or resolving outages. This functionality is restricted to users with access to manage each component.',
          side: 'bottom',
          align: 'end',
        },
      },
      {
        element: '[data-tour="page-tour-button"]',
        popover: {
          title: 'Page tour',
          description:
            'This button is available on all pages that have a tour. Use it to restart the tour for that page at any time.',
          side: 'bottom',
          align: 'end',
        },
      },
    ]
  }
  if (pathname.includes('/outages/')) {
    return [
      {
        element: '[data-tour="outage-detail-header"]',
        popover: {
          title: 'Outage details',
          description:
            'This page shows the outage summary, timing, who created or resolved it, and any automated monitoring or reporting.',
          side: 'bottom',
          align: 'center',
        },
      },
      {
        element: '[data-tour="outage-detail-actions"]',
        popover: {
          title: 'Actions menu',
          description: 'Click the Actions button to open the menu, then click Next.',
          side: 'bottom',
          align: 'end',
        },
      },
      {
        element: '[data-tour="outage-action-update"]',
        waitForTarget: false,
        popover: {
          title: 'Update outage',
          description: 'Edit this outage’s details (description, severity, etc.).',
          side: 'right',
          align: 'start',
        },
      },
      {
        element: '[data-tour="outage-action-resolve"]',
        waitForTarget: false,
        popover: {
          title: 'Resolve outage',
          description: 'Set the end time and mark this outage resolved.',
          side: 'right',
          align: 'start',
        },
      },
    ]
  }
  if (pathname.startsWith('/pages/')) {
    return [
      {
        element: '[data-tour="external-page-content"]',
        popover: {
          title: 'Statistical Process Controls',
          description:
            'This dashboard uses control charts to monitor process stability over time, detecting anomalies and ensuring SHIP metrics remain within expected bounds. Content is refreshed periodically.',
          side: 'top' as const,
          align: 'center' as const,
        },
      },
    ]
  }
  if (segments.length === 2 && !pathname.startsWith('/tags')) {
    return [
      {
        element: '[data-tour="subcomponent-detail-header"]',
        popover: {
          title: 'Sub-component page',
          description: 'This page shows the sub-component status and its outage history.',
          side: 'bottom',
          align: 'center',
        },
      },
      {
        element: '[data-tour="subcomponent-detail-component-link"]',
        popover: {
          title: 'Component details',
          description:
            "Navigate to the component details for this sub-component's parent component.",
          side: 'bottom',
          align: 'end',
        },
      },
      {
        element: '[data-tour="subcomponent-detail-filter"]',
        popover: {
          title: 'Outage filter',
          description: 'Filter the list by All, Ongoing, or Resolved outages.',
          side: 'bottom',
          align: 'end',
        },
      },
      {
        element: '[data-tour="subcomponent-detail-grid"]',
        popover: {
          title: 'Outages',
          description:
            'Outages for this sub-component. Use the Details button on a row to open the full outage.',
          side: 'top',
          align: 'center',
        },
      },
      {
        element: '[data-tour="subcomponent-report-outage"]',
        waitForTarget: false,
        popover: {
          title: 'Report outage',
          description:
            'Report a new outage for this sub-component. Opens a form to describe and submit it.',
          side: 'left',
          align: 'center',
        },
      },
      {
        element: '[data-tour="subcomponent-detail-grid"] [data-tour="outage-actions"]',
        waitForTarget: false,
        popover: {
          title: 'Update and resolve',
          description: 'Use the Actions button on a row to update an outage or resolve it.',
          side: 'left',
          align: 'center',
        },
      },
    ]
  }
  if (segments.length === 1) {
    return []
  }
  return []
}

function handleCloseClick(
  _element: Element | undefined,
  _step: DriveStep,
  options: { driver: ReturnType<typeof driver> },
) {
  options.driver.destroy()
}

// useTourTargetsReady is a hook that waits for the tour targets to be ready.
// This is necessary as the header loads before the rest of the page.
function useTourTargetsReady(pathname: string): boolean {
  const selectorsToWait = useMemo(() => {
    const steps = getStepsForRoute(pathname)
    const toWait = steps.filter((s) => s.waitForTarget !== false)
    return toWait.map((s) => String(s.element))
  }, [pathname])
  const [pathnameReady, setPathnameReady] = useState<string | null>(null)

  useEffect(() => {
    if (typeof document === 'undefined' || selectorsToWait.length === 0) {
      queueMicrotask(() => setPathnameReady(pathname))
      return
    }
    const allExist = () => selectorsToWait.every((sel) => document.querySelector(sel))
    if (allExist()) {
      queueMicrotask(() => setPathnameReady(pathname))
      return
    }
    const observer = new MutationObserver(() => {
      if (allExist()) {
        setPathnameReady(pathname)
        observer.disconnect()
      }
    })
    observer.observe(document.body, { childList: true, subtree: true })
    return () => observer.disconnect()
  }, [pathname, selectorsToWait])

  return pathnameReady === pathname
}

function AppTour() {
  const location = useLocation()
  const driverRef = useRef<ReturnType<typeof driver> | null>(null)
  const tourTargetsReady = useTourTargetsReady(location.pathname)

  useEffect(() => {
    if (!tourTargetsReady) return
    const routeType = getRouteType(location.pathname)
    const alreadySeen = routeType ? getSeenRouteTypes().includes(routeType) : false

    if (alreadySeen) return

    const steps = stepsWithExistingTargets(getStepsForRoute(location.pathname))
    if (steps.length === 0) return

    const driverObj = driver(buildDriverConfig(steps, routeType))
    driverRef.current = driverObj
    driverObj.drive()

    return () => {
      if (driverRef.current?.isActive()) {
        driverRef.current.destroy()
      }
      driverRef.current = null
    }
  }, [location.pathname, tourTargetsReady])

  useEffect(() => {
    const handleRestart = () => runTourForCurrentPage()
    window.addEventListener(TOUR_RESTART_EVENT, handleRestart)
    return () => window.removeEventListener(TOUR_RESTART_EVENT, handleRestart)
  }, [])

  return null
}

export function useHasTour(): boolean {
  const location = useLocation()
  return getStepsForRoute(location.pathname).length > 0
}

export default AppTour
