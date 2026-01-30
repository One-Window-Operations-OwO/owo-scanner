'use client';

import { useState } from 'react';

// --- Interface Data ---
interface ScanResponse {
  success: boolean;
  image?: string;
  message?: string;
}

export default function Home() {
  const [imageSrc, setImageSrc] = useState<string | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [status, setStatus] = useState<{ type: 'idle' | 'success' | 'error'; msg: string }>({
    type: 'idle',
    msg: '',
  });

  const handleScan = async () => {
    setLoading(true);
    setStatus({ type: 'idle', msg: '⏳ Menghubungkan ke Scanner...' });
    setImageSrc(null);

    try {
      const response = await fetch('http://localhost:5000/scan', {
        method: 'GET',
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data: ScanResponse = await response.json();

      if (data.success && data.image) {
        setImageSrc(data.image);
        setStatus({ type: 'success', msg: '✅ Scan Berhasil!' });
      } else {
        setStatus({ type: 'error', msg: `❌ Gagal: ${data.message || 'Unknown error'}` });
      }

    } catch (error) {
      console.error(error);
      setStatus({
        type: 'error',
        msg: '⚠️ Error: Pastikan aplikasi Bridge (.exe) sudah jalan!'
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
      {/* Main Card */}
      <div className="w-full max-w-2xl bg-white rounded-xl shadow-lg overflow-hidden border border-gray-100">

        {/* Header */}
        <div className="bg-slate-800 p-6 text-white text-center">
          <h1 className="text-2xl font-bold tracking-wide">Scanner Dokumentasi Sekolah</h1>
          <p className="text-slate-300 text-sm mt-1">Plustek PS3180U Integrator</p>
        </div>

        {/* Content */}
        <div className="p-8 space-y-6">

          {/* Status Indicator */}
          {status.msg && (
            <div className={`p-4 rounded-lg text-sm font-medium border ${status.type === 'error' ? 'bg-red-50 text-red-700 border-red-200' :
              status.type === 'success' ? 'bg-green-50 text-green-700 border-green-200' :
                'bg-blue-50 text-blue-700 border-blue-200'
              }`}>
              {status.msg}
            </div>
          )}

          {/* Action Button */}
          <button
            onClick={handleScan}
            disabled={loading}
            className={`w-full py-4 px-6 rounded-lg font-bold text-white shadow-md transition-all transform hover:scale-[1.01] active:scale-[0.99] flex items-center justify-center gap-2 ${loading
              ? 'bg-gray-400 cursor-not-allowed opacity-75'
              : 'bg-blue-600 hover:bg-blue-700'
              }`}
          >
            {loading ? (
              <>
                <svg className="animate-spin h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span>Sedang Scanning...</span>
              </>
            ) : (
              <>
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"></path>
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"></path>
                </svg>
                <span>MULAI SCAN DOKUMEN</span>
              </>
            )}
          </button>

          {/* Preview Area */}
          <div className={`mt-6 border-2 border-dashed rounded-xl flex flex-col items-center justify-center min-h-[300px] transition-colors ${imageSrc ? 'border-blue-300 bg-blue-50/30' : 'border-gray-300 bg-gray-50'
            }`}>
            {imageSrc ? (
              <div className="w-full p-4 flex flex-col items-center">
                <img
                  src={imageSrc}
                  alt="Preview Scan"
                  className="max-w-full max-h-[500px] rounded-lg shadow-md border border-gray-200"
                />
                <div className="mt-6 flex gap-3 w-full justify-center">
                  <a
                    href={imageSrc}
                    download={`scan_${new Date().getTime()}.jpg`}
                    className="px-4 py-2 bg-white border border-gray-300 text-gray-700 font-medium rounded-lg hover:bg-gray-50 shadow-sm text-sm"
                  >
                    ⬇️ Download JPG
                  </a>
                  {/* Tombol dummy buat upload nanti */}
                  <button className="px-4 py-2 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 shadow-sm text-sm">
                    💾 Simpan ke Database
                  </button>
                </div>
              </div>
            ) : (
              <div className="text-center p-6 text-gray-400">
                <svg className="mx-auto h-12 w-12 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                </svg>
                <p>Belum ada hasil scan.</p>
                <p className="text-xs mt-1">Tekan tombol di atas untuk memulai.</p>
              </div>
            )}
          </div>

        </div>
      </div>
    </div>
  );
}