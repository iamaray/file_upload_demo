'use client';

import { useCallback, useMemo, useRef, useState } from "react";
import Papa from 'papaparse';

type UploadResponse = {
    id: string;
    bytesWritten: number;
    sha256: string;
    contentType: string;
    filename: string;
}

const PREVIEW_ROWS = 200;

export default function UploadAndPreview() {
    const [dragOver, setDragOver] = useState(false);
    const [progress, setProgress] = useState<number | null>(null);
    const [status, setStatus] = useState<'idle'|'uploading'|'done'|'error'>('idle');
    const [message, setMessage] = useState<string>('');
    const [hasHeader, setHasHeader] = useState<boolean>(false);
  
    const [rows, setRows] = useState<string[][]>([]);
    const [columns, setColumns] = useState<string[]>([]);
    const fileRef = useRef<HTMLInputElement>(null);

    const handleFiles = useCallback(async (file: File) => {
        setMessage('');
        setProgress(0);
        setStatus('uploading');
        setRows([]);
        setColumns([]);

        await new Promise<void>((resolve) => {
            Papa.parse<string[]>(file, {
                worker: true,
                preview: PREVIEW_ROWS + 1,
                skipEmptyLines: 'greedy', 
                dynamicTyping: false,
                complete: (res) => {
                    const data = (res.data as unknown as string[][]).filter(r => r.length > 0);
                    if (data.length === 0) {
                        setMessage('No rows detected in CSV.');
                    }
                    applyHeaderSetting(data, hasHeader);
                    resolve();
                },
                error: (err) => {
                    setStatus('error');
                    setMessage(`CSV pase error: ${err.message}`);
                    resolve();
                },
            });
        });

        try {
            const resp = await uploadWithProgress('/api/files/', file, setProgress);
            setStatus('done');
            setMessage(`Uploaded as ${resp.filename} (${resp.bytesWritten} bytes)`);
        } catch (e: any) {
            setStatus('error');
            setMessage(e?.message || 'Upload Failed');
        }
    }, [hasHeader]);

    const applyHeaderSetting = (data: string[][], header: boolean) => {
        if (!data || data.length === 0) {
            setRows([]);
            setColumns([]);
            return;
        }
        if (header) {
            const hdr = data[0] ?? [];
            const cols = hdr.map((c,i) => (c && c.trim().length ? c : `col_${i+1}`));
            setColumns(cols);
            setRows(data.slice(1, PREVIEW_ROWS + 1));
        } else {
            const maxW = Math.max(...data.slice(0, PREVIEW_ROWS).map(r => r.length));
            const cols = Array.from({length: maxW}, (_, i) => `col_${i+1}`);
            setColumns(cols);
            setRows(data.slice(0, PREVIEW_ROWS));
        }
    };

    const onHeaderToggle = () => {
        setHasHeader (h => {
            const next = !h;
            const data = (hasHeader ? [columns, ...rows]: rows).slice();

            if (fileRef.current?.files?.[0]) {
                Papa.parse<string[]>(fileRef.current.files[0], {
                    worker: true,
                    preview: PREVIEW_ROWS + 1, 
                    skipEmptyLines: 'greedy',
                    complete: (res) => {
                        applyHeaderSetting(res.data as unknown as string[][], next);
                    },
                });
            }
            return next;
        });  
    };

    const onDrop = (e : React.DragEvent<HTMLDivElement>) => {
        e.preventDefault();
        setDragOver(false);
        const file = e.dataTransfer.files?.[0];

        if (file) {
            if (!file.name.toLowerCase().endsWith('.csv')) {
                setStatus('error');
                setMessage('Please drop a .csv file.')
                return;
            }
            void handleFiles(file);
        }
    };

    const onBrowse = () => fileRef.current?.click();
    const onFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const f = e.target.files?.[0];
        if (f) void handleFiles(f);
    };

    const table = useMemo(() => {
        if (rows.length === 0) return null;
        return (
            <div className="w-full overflow-auto border rounded-md">
                <table className="min-w-full text-sm">
                    <thead className="sticky top-0 bg-white">
                        <tr>
                            {columns.map((c, i) => (
                                <th key={i} className="px-3 py-2 border-b text-left font-medium">{c}</th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((r, ri) => (
                            <tr key={ri} className="odd:bg-gray-50">
                                {columns.map((_, ci) => (
                                <td key={ci} className="px-3 py-2 border-b whitespace-nowrap">
                                    {r[ci] ?? ''}
                                </td>
                                ))}
                            </tr>
                            ))}
                    </tbody>
                </table>
            </div>
        );
    }, [columns, rows]);

    return (
        <div className="w-full max-w-4xl flex flex-col gap-4">
            <div
                onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
                onDragLeave={() => setDragOver(false)}
                onDrop={onDrop}
                className={[
                    'w-full rounded-xl border-2 border-dashed p-10 text-center transition',
                    dragOver ? 'border-blue-500 bg-blue-50' : 'border-gray-300',
                ].join('')}
            >
                <p className="mb-2">Drag & drop a CSV here</p>
                <p className="mb-4 text-gray-500">or</p>

                <button
                    onClick={onBrowse}
                    className="px-4 py-2 rounded-lg border bg-white hover:bg-gray-50"
                >
                    Browse…
                </button>
                
                <input
                    ref={fileRef}
                    type="file"
                    accept=".csv,text/csv"
                    className="hidden"
                    onChange={onFileChange}
                />
            </div>

            {progress !== null && (
                <div className="w-full">
                    <div className="mb-1 text-sm text-gray-600">
                        {status === 'uploading' ? 'Uploading…' : status === 'done' ? 'Upload complete' : 'Status'}
                    </div>

                    <div className="h-2 bg-gray-200 rounded">
                        <div
                        className="h-2 bg-blue-600 rounded"
                        style={{ width: `${Math.round((progress ?? 0) * 100)}%` }}
                        />
                    </div>
                </div>
            )}

            <div className="flex items-center gap-3">
                <label className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={hasHeader} onChange={onHeaderToggle} />
                        First row is header
                </label>

                <span className="text-sm text-gray-500">Showing first {PREVIEW_ROWS} rows</span>
            </div>

            {message && (
                <div className={`text-sm ${status === 'error' ? 'text-red-600' : 'text-gray-700'}`}>
                {message}
                </div>
            )}

            {table}
        </div>
    )
};

function uploadWithProgress(endpoint: string, file: File, onProgress: (p:number) => void) {
    return new Promise<UploadResponse>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', endpoint); 

        xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) onProgress(e.loaded / e.total);
        };
        xhr.onerror = () => reject(new Error('Network error'));
        xhr.ontimeout = () => reject(new Error('Request timed out'));

        xhr.onload = () => {
            try {
                if (xhr.status >= 200 && xhr.status < 300) {
                    resolve(JSON.parse(xhr.responseText) as UploadResponse);
                } else {
                    reject(new Error(`Server error ${xhr.status}: ${xhr.responseText}`));
                }
            } catch (e) {
                reject(new Error('Invalid JSON response'));
            }
        };

        const form = new FormData();
        form.append('file', file);
        xhr.send(form);
    });
}