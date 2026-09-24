import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import ErrorBoundary from './components/ErrorBoundary'
import {LogFrontendError} from '../wailsjs/go/main/App'

window.addEventListener('error', (e) => {
  LogFrontendError(e.message, e.error?.stack ?? `${e.filename}:${e.lineno}:${e.colno}`).catch(() => {})
})
window.addEventListener('unhandledrejection', (e) => {
  const r = e.reason
  LogFrontendError(r instanceof Error ? r.message : String(r), r instanceof Error ? r.stack ?? '' : '').catch(() => {})
})

const container = document.getElementById('root')
if (container) {
  container.style.height = '100%'
}

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <ErrorBoundary>
            <App/>
        </ErrorBoundary>
    </React.StrictMode>
)
