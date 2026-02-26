import { Edit, MoreVert } from '@mui/icons-material'
import { Button, ListItemIcon, ListItemText, Menu, MenuItem, Tooltip } from '@mui/material'
import type { MouseEvent } from 'react'
import { useState } from 'react'

import { useAuth } from '../../../contexts/AuthContext'
import type { Outage } from '../../../types'

import ConfirmOutage from './ConfirmOutage'
import DeleteOutage from './DeleteOutage'
import EndOutage from './EndOutage'
import UpsertOutageModal from './UpsertOutageModal'

interface OutageActionsProps {
  outage: Outage
  onSuccess: () => void
  onError: (error: string) => void
}

const OutageActions = ({ outage, onSuccess, onError }: OutageActionsProps) => {
  const { isComponentAdmin } = useAuth()
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null)
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false)

  const isAdmin = isComponentAdmin(outage.component_name)

  if (!isAdmin) {
    return null
  }

  const handleMenuClick = (event: MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget)
  }

  const handleMenuClose = () => {
    setAnchorEl(null)
  }

  const handleUpdateClick = () => {
    setUpdateDialogOpen(true)
    handleMenuClose()
  }

  const handleUpdateClose = () => {
    setUpdateDialogOpen(false)
  }

  return (
    <>
      <Tooltip title="Outage actions" arrow>
        <span style={{ display: 'inline-flex' }} data-tour="outage-actions">
          <Button size="small" onClick={handleMenuClick} startIcon={<MoreVert />}>
            Actions
          </Button>
        </span>
      </Tooltip>

      <Menu
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleMenuClose}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
      >
        <MenuItem onClick={handleUpdateClick} data-tour="outage-action-update">
          <ListItemIcon>
            <Edit fontSize="small" />
          </ListItemIcon>
          <ListItemText>Update</ListItemText>
        </MenuItem>
        {!outage.confirmed_at.Valid && (
          <MenuItem>
            <ConfirmOutage outage={outage} onConfirmSuccess={onSuccess} onError={onError} />
          </MenuItem>
        )}
        {!outage.end_time.Valid && (
          <MenuItem data-tour="outage-action-resolve">
            <EndOutage outage={outage} onEndSuccess={onSuccess} onError={onError} />
          </MenuItem>
        )}
        <MenuItem>
          <DeleteOutage outage={outage} onDeleteSuccess={onSuccess} onError={onError} />
        </MenuItem>
      </Menu>

      <UpsertOutageModal
        open={updateDialogOpen}
        onClose={handleUpdateClose}
        onSuccess={onSuccess}
        componentName={outage.component_name}
        subComponentName={outage.sub_component_name}
        outage={outage}
      />
    </>
  )
}

export default OutageActions
