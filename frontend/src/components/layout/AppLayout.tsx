import { useState, ReactNode } from 'react'
import { Sidebar } from '@/components/layout/Sidebar'
import { Header } from '@/components/layout/Header'
import { UploadModal } from '@/components/layout/UploadModal'
import { X } from 'lucide-react'

interface AppLayoutProps {
  children: ReactNode
  uploadModalOpen?: boolean
  onOpenUploadModal?: () => void
  onCloseUploadModal?: () => void
  onUploadSuccess?: () => void
}

export function AppLayout({
  children,
  uploadModalOpen: controlledModalOpen,
  onOpenUploadModal,
  onCloseUploadModal,
  onUploadSuccess,
}: AppLayoutProps) {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [internalModalOpen, setInternalModalOpen] = useState(false)

  const isModalOpen = controlledModalOpen !== undefined ? controlledModalOpen : internalModalOpen
  const handleOpenModal = onOpenUploadModal || (() => setInternalModalOpen(true))
  const handleCloseModal = onCloseUploadModal || (() => setInternalModalOpen(false))

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col">
      {/* Desktop Sidebar */}
      <div className="hidden md:fixed md:inset-y-0 md:left-0 md:z-40 md:flex md:w-64 md:flex-col">
        <Sidebar onUploadClick={handleOpenModal} />
      </div>

      {/* Mobile Drawer Backdrop and Sidebar */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-50 md:hidden flex">
          <div
            className="fixed inset-0 bg-black/60 backdrop-blur-sm transition-opacity"
            onClick={() => setMobileMenuOpen(false)}
          />
          <div className="relative flex w-4/5 max-w-xs flex-1 flex-col bg-card shadow-2xl z-10">
            <button
              onClick={() => setMobileMenuOpen(false)}
              aria-label="Close navigation menu"
              className="absolute right-3 top-3 p-2 rounded-lg text-muted-foreground hover:bg-muted"
            >
              <X className="w-5 h-5" />
            </button>
            <Sidebar
              onUploadClick={handleOpenModal}
              onCloseMobile={() => setMobileMenuOpen(false)}
            />
          </div>
        </div>
      )}

      {/* Main Content Area */}
      <div className="flex flex-1 flex-col md:pl-64">
        <Header
          onOpenMobileMenu={() => setMobileMenuOpen(true)}
          onUploadClick={handleOpenModal}
        />
        <main className="flex-1 p-4 md:p-8 max-w-7xl w-full mx-auto">
          {children}
        </main>
      </div>

      {/* Photo Upload Modal */}
      <UploadModal
        isOpen={isModalOpen}
        onClose={handleCloseModal}
        onUploadSuccess={onUploadSuccess}
      />
    </div>
  )
}
