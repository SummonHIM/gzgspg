export interface WorkerStatusPayload {
  running: boolean;
  status:
    | "Starting"
    | "NotLoggedIn"
    | "LoggingIn"
    | "LoggedIn"
    | "Paused"
    | "LoggingOut"
    | "Stopped";
  message?: string | null;
}

export interface StartWorkerResponse {
  started: boolean
  alreadyRunning: boolean
  state: WorkerStatusPayload
}

