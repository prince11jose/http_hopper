'use client'

import { useState, useEffect, useRef } from 'react'
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ArrowRight } from 'lucide-react'

const MAX_FILE_SIZE = 1024 * 1024 // 1MB

export function WebtrafficLogger() {
  const [url, setUrl] = useState('')
  const [connected, setConnected] = useState(false)
  const [messages, setMessages] = useState<string[]>([])
  const [fileCount, setFileCount] = useState(0)
  const ws = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages])

  const connectWebSocket = () => {
    if (ws.current) {
      ws.current.close()
    }

    ws.current = new WebSocket(url)

    ws.current.onopen = () => {
      setConnected(true)
      console.log('WebSocket Connected')
    }

    ws.current.onmessage = (event) => {
      setMessages(prev => [event.data, ...prev])
      saveMessageToStorage(event.data)
    }

    ws.current.onclose = () => {
      setConnected(false)
      console.log('WebSocket Disconnected')
    }
  }

  const saveMessageToStorage = (message: string) => {
    const currentData = localStorage.getItem(`webtraffic_log_${fileCount}`) || ''
    const newData = message + '\n' + currentData

    if (new Blob([newData]).size > MAX_FILE_SIZE) {
      setFileCount(prev => prev + 1)
      localStorage.setItem(`webtraffic_log_${fileCount + 1}`, message + '\n')
    } else {
      localStorage.setItem(`webtraffic_log_${fileCount}`, newData)
    }
  }

  const exportLogs = async () => {
    try {
      const fileHandle = await window.showSaveFilePicker({
        suggestedName: 'webtraffic_logs.log',
        types: [{
          description: 'Log File',
          accept: { 'text/plain': ['.log'] },
        }],
      })
      const writable = await fileHandle.createWritable()
      
      for (let i = fileCount; i >= 0; i--) {
        const data = localStorage.getItem(`webtraffic_log_${i}`)
        if (data) {
          await writable.write(data)
        }
      }
      
      await writable.close()
      console.log('WebTraffic logs exported successfully')
    } catch (err) {
      console.error('Failed to export WebTraffic logs:', err)
    }
  }

  return (
    <div className="flex flex-col h-screen w-screen bg-gray-100 dark:bg-gray-900 text-gray-900 dark:text-gray-100">
      <div className="flex justify-between items-center p-4 bg-white dark:bg-gray-800 shadow">
        <h1 className="text-2xl font-bold">WebTraffic Logger</h1>
        <Button onClick={exportLogs} disabled={!messages.length} className="ml-auto">
          Export Logs
        </Button>
      </div>
      <div className="flex-grow flex flex-col p-4 space-y-4 overflow-hidden">
        <div className="flex space-x-2">
          <Input
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="Enter WebSocket URL (e.g., ws://localhost:8000/traffic)"
            className="flex-grow"
            aria-label="WebSocket URL"
          />
          <Button onClick={connectWebSocket} disabled={!url || connected}>
            {connected ? 'Connected' : 'Connect'}
          </Button>
        </div>
        <div className="flex-grow overflow-y-auto bg-white dark:bg-gray-800 rounded-lg shadow p-4">
          {!connected && messages.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-center space-y-4">
              <h2 className="text-xl font-semibold">Welcome to WebTraffic Logger</h2>
              <p className="max-w-md">To get started, follow these steps:</p>
              <ol className="list-decimal text-left space-y-2 max-w-md">
                <li>Enter a valid WebSocket URL in the input field above.</li>
                <li>Click the "Connect" button to establish a connection.</li>
                <li>Once connected, incoming messages will appear here.</li>
                <li>Use the "Export Logs" button to save your logs when needed.</li>
              </ol>
              <ArrowRight className="animate-bounce text-primary mt-4" size={32} />
            </div>
          ) : (
            messages.map((msg, index) => (
              <div key={index} className="mb-2 text-sm">
                {msg}
              </div>
            ))
          )}
          <div ref={messagesEndRef} />
        </div>
      </div>
    </div>
  )
}