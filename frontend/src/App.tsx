import { useState } from 'react'
import { Routes, Route, useNavigate } from 'react-router-dom'
import { AppLayout } from '@/components/layout/AppLayout'
import { PhotosPage } from '@/pages/PhotosPage'
import { TimelinePage } from '@/pages/TimelinePage'
import { FoldersPage } from '@/pages/FoldersPage'
import { LightboxModal } from '@/components/gallery/LightboxModal'
import { Photo } from '@/types/photo'
import { Folder } from '@/types/folder'
import { MOCK_PHOTOS } from '@/data/mock/photos'

export function App() {
  const navigate = useNavigate()
  const [selectedPhoto, setSelectedPhoto] = useState<Photo | null>(null)
  const [activePhotosList, setActivePhotosList] = useState<Photo[]>(MOCK_PHOTOS)
  const [isLightboxOpen, setIsLightboxOpen] = useState(false)
  const [folderFilter, setFolderFilter] = useState<string | null>(null)
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false)
  const [uploadRefreshKey, setUploadRefreshKey] = useState(0)

  const handleSelectPhoto = (photo: Photo, list: Photo[]) => {
    setSelectedPhoto(photo)
    setActivePhotosList(list)
    setIsLightboxOpen(true)
  }

  const handleSelectFolder = (folder: Folder) => {
    setFolderFilter(folder.id)
    navigate('/')
  }

  const handleClearFolderFilter = () => {
    setFolderFilter(null)
  }

  const handleUploadSuccess = () => {
    // Trigger gallery reload
    setUploadRefreshKey((k) => k + 1)
  }

  return (
    <AppLayout
      uploadModalOpen={isUploadModalOpen}
      onOpenUploadModal={() => setIsUploadModalOpen(true)}
      onCloseUploadModal={() => setIsUploadModalOpen(false)}
      onUploadSuccess={handleUploadSuccess}
    >
      <Routes>
        <Route
          path="/"
          element={
            <PhotosPage
              key={uploadRefreshKey}
              onSelectPhoto={handleSelectPhoto}
              onUploadClick={() => setIsUploadModalOpen(true)}
              folderFilter={folderFilter}
              onClearFolderFilter={handleClearFolderFilter}
            />
          }
        />
        <Route
          path="/photos"
          element={
            <PhotosPage
              key={uploadRefreshKey}
              onSelectPhoto={handleSelectPhoto}
              onUploadClick={() => setIsUploadModalOpen(true)}
              folderFilter={folderFilter}
              onClearFolderFilter={handleClearFolderFilter}
            />
          }
        />
        <Route
          path="/timeline"
          element={<TimelinePage onSelectPhoto={handleSelectPhoto} />}
        />
        <Route
          path="/folders"
          element={<FoldersPage onSelectFolder={handleSelectFolder} />}
        />
        {/* Fallback to root for any other client route */}
        <Route
          path="*"
          element={
            <PhotosPage
              key={uploadRefreshKey}
              onSelectPhoto={handleSelectPhoto}
              onUploadClick={() => setIsUploadModalOpen(true)}
            />
          }
        />
      </Routes>

      {/* Lightbox / Photo Viewer */}
      <LightboxModal
        isOpen={isLightboxOpen}
        photo={selectedPhoto}
        photos={activePhotosList}
        onClose={() => setIsLightboxOpen(false)}
        onNavigate={(nextPhoto) => setSelectedPhoto(nextPhoto)}
      />
    </AppLayout>
  )
}

export default App
