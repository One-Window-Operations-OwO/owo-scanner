'use client';

import { useState, useEffect } from 'react';

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
  const [profiles, setProfiles] = useState<string[]>([]);
  const [profileName, setProfileName] = useState<string>("");
  const [status, setStatus] = useState<{ type: 'idle' | 'success' | 'error'; msg: string }>({
    type: 'idle',
    msg: '',
  });

  const [theme, setTheme] = useState<'light' | 'dark'>('light');

  // Initialize theme
  useEffect(() => {
    // Check local storage or system preference
    const storedTheme = localStorage.getItem('theme') as 'light' | 'dark' | null;
    const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;

    if (storedTheme) {
      setTheme(storedTheme);
      document.documentElement.classList.toggle('dark', storedTheme === 'dark');
    } else if (systemPrefersDark) {
      setTheme('dark');
      document.documentElement.classList.add('dark');
    }
  }, []);

  const toggleTheme = () => {
    const newTheme = theme === 'light' ? 'dark' : 'light';
    setTheme(newTheme);
    localStorage.setItem('theme', newTheme);
    document.documentElement.classList.toggle('dark', newTheme === 'dark');
  };

  // Fetch profiles on mount
  useEffect(() => {
    const fetchProfiles = async () => {
      // Skip fetching profiles in mock mode
      if (process.env.NEXT_PUBLIC_USE_MOCK === 'true') {
        setProfiles(['Mock Profile 1', 'Mock Profile 2']);
        setProfileName('Mock Profile 1');
        return;
      }

      try {
        const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/profiles`);
        if (!res.ok) throw new Error('Failed to fetch profiles');
        const data = await res.json();
        if (data.success && data.profiles && Array.isArray(data.profiles)) {
          setProfiles(data.profiles);
          if (data.profiles.length > 0) {
            setProfileName(data.profiles[0]); // Set default to first profile
          }
        }
      } catch (err) {
        console.error("Error fetching profiles:", err);
        // Fallback or just keep empty
      }
    };

    fetchProfiles();
  }, []);

  const handleScan = async () => {
    setLoading(true);
    setStatus({ type: 'idle', msg: '⏳ Menghubungkan ke Scanner...' });
    setScanResults([]);

    try {
      const isMock = process.env.NEXT_PUBLIC_USE_MOCK === 'true';
      const apiUrl = process.env.NEXT_PUBLIC_API_URL;

      const endpoint = isMock
        ? `${apiUrl}`
        : `${apiUrl}/scan?profile=${encodeURIComponent(profileName)}`;

      const response = await fetch(endpoint, {
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
    <div className="min-h-screen bg-gray-50 dark:bg-slate-950 flex items-center justify-center p-4">
      {/* Main Card */}
      <div className="w-full max-w-4xl bg-white dark:bg-slate-900 rounded-xl shadow-lg overflow-hidden border border-gray-100 dark:border-slate-800">

        {/* Header */}
        <div className="bg-slate-800 p-6 text-white flex justify-between items-center">
          <h1 className="text-2xl font-bold tracking-wide">Scanner Dokumentasi Sekolah</h1>
          <button
            onClick={toggleTheme}
            className="p-2 rounded-full bg-slate-700 hover:bg-slate-600 transition-colors"
            aria-label="Toggle Theme"
          >
            {theme === 'light' ? (
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                <path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.72 9.72 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z" />
              </svg>
            ) : (
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z" />
              </svg>
            )}
          </button>
        </div>

        {/* Content */}
        <div className="p-8 space-y-6">

          {/* Profile Configuration */}
          <div className="bg-blue-50 dark:bg-slate-900/50 p-4 rounded-lg border border-gray-100 dark:border-gray-800">
            <label htmlFor="profileName" className="block text-sm font-medium text-black dark:text-gray-200 mb-2">
              Pilih Profil NAPS2
            </label>
            {profiles.length > 0 ? (
              <select
                id="profileName"
                value={profileName}
                onChange={(e) => setProfileName(e.target.value)}
                className="w-full px-4 py-2 border border-blue-200 dark:border-blue-800/50 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none text-gray-700 dark:text-gray-200 bg-white dark:bg-slate-800"
              >
                {profiles.map((p, idx) => (
                  <option key={idx} value={p}>{p}</option>
                ))}
              </select>
            ) : (
              <div className="flex gap-2">
                <input
                  type="text"
                  id="profileName"
                  value={profileName}
                  onChange={(e) => setProfileName(e.target.value)}
                  className="w-full px-4 py-2 border border-blue-200 dark:border-blue-800/50 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none text-gray-700 dark:text-gray-200 bg-white dark:bg-slate-800"
                  placeholder="Nama Profile (Manual)"
                />
                <span className="text-xs text-gray-400 self-center whitespace-nowrap">
                  (Gagal load profiles)
                </span>
              </div>
            )}
            <p className="text-xs text-blue-600 mt-2">
              Profil diambil dari konfigurasi NAPS2 di komputer ini.
            </p>
          </div>

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
          <div className={`mt-6 transition-colors ${scanResults.length > 0 ? '' : 'border-2 border-dashed border-gray-300 dark:border-slate-700 bg-gray-50 dark:bg-slate-900/50 rounded-xl min-h-[200px] flex items-center justify-center'}`}>

            {scanResults.length > 0 ? (
              <div className="space-y-6">
                {scanResults.map((pair, index) => (
                  <div key={index} className="bg-gray-50 dark:bg-slate-800/50 p-4 rounded-xl border border-gray-200 dark:border-slate-700">
                    <div className="flex justify-between items-center mb-3">
                      <span className="font-semibold text-gray-700 dark:text-gray-200">Dokumen #{index + 1}</span>
                      <button className="text-xs bg-green-100 text-green-700 px-3 py-1 rounded-full font-medium border border-green-200 hover:bg-green-200">
                        💾 Simpan Set Ini
                      </button>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      {/* Front Image */}
                      <div className="flex flex-col gap-2">
                        <img
                          src={pair.front}
                          alt={`Front ${index + 1}`}
                          className="w-full h-auto rounded-lg shadow-sm border border-gray-300 dark:border-slate-600 bg-white dark:bg-slate-900"
                        />
                      </div>

                      {/* Back Image (if exists) */}
                      {pair.back ? (
                        <div className="flex flex-col gap-2">
                          <img
                            src={pair.back}
                            alt={`Back ${index + 1}`}
                            className="w-full h-auto rounded-lg shadow-sm border border-gray-300 bg-white"
                          />
                        </div>
                      ) : (
                        <div className="flex flex-col gap-2 opacity-50">
                          <span className="text-xs font-medium text-gray-500 uppercase tracking-wider text-center">Halaman Belakang</span>
                          <div className="w-full h-full min-h-[200px] flex items-center justify-center bg-gray-200 dark:bg-slate-800 rounded-lg border border-dashed border-gray-400 dark:border-slate-600">
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