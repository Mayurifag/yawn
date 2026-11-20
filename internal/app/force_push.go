package app

func (a *App) RunForcePush() error {
	if err := a.ensureSSHRemote(); err != nil {
		return err
	}
	return a.pushChanges(true, a.Config.AutoPush)
}
