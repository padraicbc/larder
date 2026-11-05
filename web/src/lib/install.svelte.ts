// Chrome fires `beforeinstallprompt` when the app can be installed. We keep the
// event so Settings can offer an "Install app" button.
type InstallEvent = Event & { prompt: () => Promise<void>; userChoice: Promise<{ outcome: string }> };

export const install = $state<{ event: InstallEvent | null; installed: boolean }>({
	event: null,
	installed: typeof matchMedia !== 'undefined' && matchMedia('(display-mode: standalone)').matches
});

if (typeof window !== 'undefined') {
	window.addEventListener('beforeinstallprompt', (e) => {
		e.preventDefault();
		install.event = e as InstallEvent;
	});
	window.addEventListener('appinstalled', () => {
		install.event = null;
		install.installed = true;
	});
}

export async function promptInstall() {
	if (!install.event) return;
	await install.event.prompt();
	await install.event.userChoice;
	install.event = null;
}
