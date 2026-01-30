'use client';

import { useState } from 'react';

// --- Interface Data ---
interface ScanPair {
  front: string;
  back?: string;
}

interface ScanResponse {
  success: boolean;
  data?: ScanPair[];
  message?: string;
}

export default function Home() {
  const [scanResults, setScanResults] = useState<ScanPair[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [status, setStatus] = useState<{ type: 'idle' | 'success' | 'error'; msg: string }>({
    type: 'idle',
    msg: '',
  });

  const handleScan = async () => {
    setLoading(true);
    setStatus({ type: 'idle', msg: '⏳ Menghubungkan ke Scanner...' });
    setScanResults([]);

    try {
      const response = await fetch('http://localhost:5000/scan', {
        method: 'GET',
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data: ScanResponse = await response.json();

      if (data.success && data.data) {
        setScanResults(data.data);
        setStatus({ type: 'success', msg: `✅ Scan Berhasil! ${data.data.length} dokumen ditemukan.` });
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
      <div className="w-full max-w-4xl bg-white rounded-xl shadow-lg overflow-hidden border border-gray-100">

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

          {/* Results Area */}
          <div className={`mt-6 transition-colors ${scanResults.length > 0 ? '' : 'border-2 border-dashed border-gray-300 bg-gray-50 rounded-xl min-h-[200px] flex items-center justify-center'}`}>

            {scanResults.length > 0 ? (
              <div className="space-y-6">
                {scanResults.map((pair, index) => (
                  <div key={index} className="bg-gray-50 p-4 rounded-xl border border-gray-200">
                    <div className="flex justify-between items-center mb-3">
                      <span className="font-semibold text-gray-700">Dokumen #{index + 1}</span>
                      <button className="text-xs bg-green-100 text-green-700 px-3 py-1 rounded-full font-medium border border-green-200 hover:bg-green-200">
                        💾 Simpan Set Ini
                      </button>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      {/* Front Image */}
                      <div className="flex flex-col gap-2">
                        <span className="text-xs font-medium text-gray-500 uppercase tracking-wider text-center">Halaman Depan</span>
                        <img
                          src={pair.front}
                          alt={`Front ${index + 1}`}
                          className="w-full h-auto rounded-lg shadow-sm border border-gray-300 bg-white"
                        />
                      </div>

                      {/* Back Image (if exists) */}
                      {pair.back ? (
                        <div className="flex flex-col gap-2">
                          <span className="text-xs font-medium text-gray-500 uppercase tracking-wider text-center">Halaman Belakang</span>
                          <img
                            src={pair.back}
                            alt={`Back ${index + 1}`}
                            className="w-full h-auto rounded-lg shadow-sm border border-gray-300 bg-white"
                          />
                        </div>
                      ) : (
                        <div className="flex flex-col gap-2 opacity-50">
                          <span className="text-xs font-medium text-gray-500 uppercase tracking-wider text-center">Halaman Belakang</span>
                          <div className="w-full h-full min-h-[200px] flex items-center justify-center bg-gray-200 rounded-lg border border-dashed border-gray-400">
                            <span className="text-gray-500 text-sm">Tidak ada halaman belakang</span>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
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