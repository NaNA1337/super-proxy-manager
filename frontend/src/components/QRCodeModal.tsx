import React, { useEffect, useRef, useState } from 'react';
import QRCode from 'qrcode';
import { Copy, Check, X, QrCode } from 'lucide-react';

interface QRCodeModalProps {
  isOpen: boolean;
  onClose: () => void;
  uri: string;
  title: string;
  protocol: string;
}

export const QRCodeModal: React.FC<QRCodeModalProps> = ({
  isOpen,
  onClose,
  uri,
  title,
  protocol,
}) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (isOpen && canvasRef.current && uri) {
      QRCode.toCanvas(canvasRef.current, uri, {
        width: 256,
        margin: 2,
        color: {
          dark: '#000000',
          light: '#ffffff',
        },
      }, (err) => {
        if (err) console.error('QR rendering error:', err);
      });
    }
  }, [isOpen, uri]);

  if (!isOpen) return null;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(uri);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (e) {
      console.error('Failed to copy link', e);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150">
      <div className="relative w-full max-w-md p-6 glass-panel rounded-xl border border-cyan-500/30 glow-cyan">
        <button
          onClick={onClose}
          className="absolute top-4 right-4 p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition"
        >
          <X className="w-5 h-5" />
        </button>

        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 rounded-lg bg-cyan-500/10 border border-cyan-500/30 text-cyan-400">
            <QrCode className="w-5 h-5" />
          </div>
          <div>
            <h3 className="text-base font-semibold text-white tracking-wide">{title}</h3>
            <span className="text-xs uppercase font-mono text-cyan-400 font-bold tracking-wider">
              {protocol} CLIENT SHARELINK
            </span>
          </div>
        </div>

        {/* Client-Side Canvas */}
        <div className="flex justify-center p-4 bg-white rounded-xl shadow-inner my-4">
          <canvas ref={canvasRef} className="rounded-lg max-w-full" />
        </div>

        <p className="text-xs text-slate-400 text-center mb-4">
          Generated entirely client-side. No URL parameters or keys were transmitted to any external API.
        </p>

        {/* URI Box */}
        <div className="relative mb-4">
          <div className="p-3 bg-noc-950/80 rounded-lg border border-slate-800 font-mono text-xs text-slate-300 break-all max-h-24 overflow-y-auto">
            {uri}
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex gap-3">
          <button
            onClick={handleCopy}
            className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-semibold text-sm rounded-lg shadow-lg shadow-cyan-900/30 transition"
          >
            {copied ? (
              <>
                <Check className="w-4 h-4 text-emerald-300" />
                <span>Copied to Clipboard!</span>
              </>
            ) : (
              <>
                <Copy className="w-4 h-4" />
                <span>Copy ShareLink</span>
              </>
            )}
          </button>
          <button
            onClick={onClose}
            className="px-4 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-sm rounded-lg transition"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
