'use client';
import UploadAndPreview from "./upload/UploadAndPreview";

export default function Page() {
  return (
    <main className="min-h-screen p-8 flex flex-col items-center gap-8">
      <h1 className="text-2xl font-semibold">CSV Upload & Preview (MVP)</h1>
      <UploadAndPreview />
    </main>
  )
}