import { Monitor, Moon, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTheme, type Theme } from '@/app/providers'

const ORDER: Theme[] = ['light', 'dark', 'system']
const ICON = { light: Sun, dark: Moon, system: Monitor } as const
const NEXT_LABEL = { light: 'dark', dark: 'system', system: 'light' } as const

/** Cycles light → dark → system, matching the OS when set to system. */
export function ThemeToggle() {
	const { theme, setTheme } = useTheme()
	const Icon = ICON[theme]

	const cycle = () => {
		const idx = ORDER.indexOf(theme)
		setTheme(ORDER[(idx + 1) % ORDER.length])
	}

	return (
		<Button
			variant="ghost"
			size="icon"
			onClick={cycle}
			aria-label={`Theme: ${theme}. Switch to ${NEXT_LABEL[theme]}.`}
			title={`Theme: ${theme}`}
		>
			<Icon />
		</Button>
	)
}
