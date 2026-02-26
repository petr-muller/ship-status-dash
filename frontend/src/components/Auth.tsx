import { Login, Person } from '@mui/icons-material'
import { Box, Button, styled, Tooltip, Typography } from '@mui/material'

import { useAuth } from '../contexts/AuthContext'
import { getProtectedDomain } from '../utils/endpoints'
import { deslugify } from '../utils/slugify'

const LoginButton = styled(Button)(({ theme }) => ({
  color: theme.palette.text.primary,
  borderColor: theme.palette.divider,
  textTransform: 'none',
  '&:hover': {
    borderColor: theme.palette.primary.main,
    backgroundColor: theme.palette.action.hover,
  },
}))

const UserDisplay = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  padding: theme.spacing(0.75, 1.5),
  borderRadius: theme.shape.borderRadius,
  backgroundColor:
    theme.palette.mode === 'dark' ? theme.palette.grey[800] : theme.palette.grey[100],
  border: `1px solid ${theme.palette.divider}`,
}))

const UserName = styled(Typography)(({ theme }) => ({
  fontSize: '0.875rem',
  color: theme.palette.text.primary,
  fontWeight: 500,
}))

const Auth = () => {
  const { user } = useAuth()

  const handleLoginClick = () => {
    window.location.href = `${getProtectedDomain()}/oauth/start`
  }

  if (user) {
    const componentList =
      user.components.length > 0 ? user.components.map(deslugify).join(', ') : 'No component access'

    return (
      <Box component="span" sx={{ display: 'inline-flex' }} data-tour="login-button">
        <UserDisplay>
          <Person fontSize="small" sx={{ color: 'text.secondary' }} />
          <Tooltip
            title={
              <Box>
                <Typography variant="caption" sx={{ display: 'block', fontWeight: 600, mb: 0.5 }}>
                  Admin of:
                </Typography>
                <Typography variant="caption">{componentList}</Typography>
              </Box>
            }
            arrow
          >
            <UserName>{user.username}</UserName>
          </Tooltip>
        </UserDisplay>
      </Box>
    )
  }

  return (
    <Box component="span" sx={{ display: 'inline-flex' }} data-tour="login-button">
      <LoginButton variant="outlined" startIcon={<Login />} onClick={handleLoginClick}>
        Login
      </LoginButton>
    </Box>
  )
}

export default Auth
