using Serilog;
using NPOI.HSSF.UserModel;
using NPOI.XSSF.UserModel;
using NPOI.SS.UserModel;
using System.IO;

namespace CRAMC_Office;

public class OfficeFileOperator {
    public async Task<SingleSanitizedDocResp> DispatchAction(SingleDocToBeSanitized doc) {
        var ssDocResp = new SingleSanitizedDocResp(doc);
        switch (doc.Action) {
            case "sanitize":
                (ssDocResp.IsSuccess, ssDocResp.AdditionalMsg) = SanitizeFile(doc.Path, doc.DestModule);
                break;
            default:
                ssDocResp.IsSuccess = false;
                ssDocResp.AdditionalMsg = "Unknown action.";
                break;
        }
        return ssDocResp;
    }
    
    private (bool isSuccessful, string addiMsg) SanitizeFile(string fPath, string mModule) {
        try {
            if (!File.Exists(fPath)) {
                return (false, "File not found.");
            }

            string fileExtension = GetFileExtension(fPath);
            if (!IsExcelFile(fileExtension)) {
                return (false, "File is not a supported Excel format.");
            }

            bool hasMacros = false;
            bool macroMatched = false;
            string additionalMessage = "";

            if (IsLegacyExcelFormat(fileExtension)) {
                (hasMacros, macroMatched, additionalMessage) = ProcessLegacyExcelFile(fPath, mModule);
            } else {
                (hasMacros, macroMatched, additionalMessage) = ProcessModernExcelFile(fPath, mModule);
            }

            if (macroMatched) {
                // Backup original file
                if (!BackupOriginalFile(fPath)) {
                    return (false, "Failed to backup original file.");
                }

                // Replace macro content
                if (!ReplaceMacroContent(fPath, fileExtension)) {
                    return (false, "Failed to replace macro content.");
                }

                // Rename file with -S suffix
                string newPath = RenameFileWithSuffix(fPath);
                if (string.IsNullOrEmpty(newPath)) {
                    return (false, "Failed to rename sanitized file.");
                }

                return (true, $"File sanitized successfully. New file: {Path.GetFileName(newPath)}. {additionalMessage}");
            } else if (hasMacros) {
                return (true, $"File contains macros, but none matched the target module '{mModule}'. {additionalMessage}");
            } else {
                return (true, $"No macros found in the file. {additionalMessage}");
            }
        } catch (Exception ex) {
            Log.Error(ex, "Error sanitizing file: {FilePath}", fPath);
            return (false, $"Error processing file: {ex.Message}");
        }
    }

    private string GetFileExtension(string filePath) {
        return Path.GetExtension(filePath).ToLowerInvariant();
    }

    private bool IsExcelFile(string extension) {
        return extension == ".xls" || extension == ".xlsx" || extension == ".xlsm" || extension == ".xlsb";
    }

    private bool IsLegacyExcelFormat(string extension) {
        return extension == ".xls";
    }

    private (bool hasMacros, bool macroMatched, string message) ProcessLegacyExcelFile(string filePath, string targetModule) {
        try {
            using var fileStream = new FileStream(filePath, FileMode.Open, FileAccess.Read);
            var workbook = new HSSFWorkbook(fileStream);
            
            // Check for VBA project
            var vbaProject = workbook.GetVBAProject();
            if (vbaProject == null) {
                return (false, false, "No VBA project found.");
            }

            var macroNames = GetMacroModuleNames(vbaProject);
            bool macroMatched = macroNames.Any(name => name.Contains(targetModule, StringComparison.OrdinalIgnoreCase));
            
            string message = macroNames.Any() ? 
                $"Found macro modules: {string.Join(", ", macroNames)}" : 
                "VBA project exists but no macro modules found.";

            return (macroNames.Any(), macroMatched, message);
        } catch (Exception ex) {
            Log.Warning(ex, "Error processing legacy Excel file: {FilePath}", filePath);
            return (false, false, $"Error reading legacy Excel file: {ex.Message}");
        }
    }

    private (bool hasMacros, bool macroMatched, string message) ProcessModernExcelFile(string filePath, string targetModule) {
        try {
            using var fileStream = new FileStream(filePath, FileMode.Open, FileAccess.Read);
            var workbook = new XSSFWorkbook(fileStream);
            
            // Check for VBA project in modern Excel format
            var vbaProject = workbook.GetVBAProject();
            if (vbaProject == null) {
                return (false, false, "No VBA project found.");
            }

            var macroNames = GetMacroModuleNames(vbaProject);
            bool macroMatched = macroNames.Any(name => name.Contains(targetModule, StringComparison.OrdinalIgnoreCase));
            
            string message = macroNames.Any() ? 
                $"Found macro modules: {string.Join(", ", macroNames)}" : 
                "VBA project exists but no macro modules found.";

            return (macroNames.Any(), macroMatched, message);
        } catch (Exception ex) {
            Log.Warning(ex, "Error processing modern Excel file: {FilePath}", filePath);
            return (false, false, $"Error reading modern Excel file: {ex.Message}");
        }
    }

    private List<string> GetMacroModuleNames(object vbaProject) {
        var moduleNames = new List<string>();
        
        try {
            // Note: NPOI's VBA project access is limited
            // This is a simplified approach - actual implementation may need more specific VBA handling
            if (vbaProject != null) {
                // In a real implementation, you would need to parse the VBA project structure
                // For now, we'll use reflection to try to get module information
                var projectType = vbaProject.GetType();
                var modules = projectType.GetProperty("Modules")?.GetValue(vbaProject);
                
                if (modules != null) {
                    // This would need to be adapted based on actual NPOI VBA project structure
                    moduleNames.Add("Module1"); // Placeholder - actual implementation would extract real module names
                }
            }
        } catch (Exception ex) {
            Log.Warning(ex, "Error extracting macro module names");
        }
        
        return moduleNames;
    }

    private bool BackupOriginalFile(string filePath) {
        try {
            string directory = Path.GetDirectoryName(filePath);
            string backupDirectory = Path.Combine(directory, "fBak");
            
            if (!Directory.Exists(backupDirectory)) {
                Directory.CreateDirectory(backupDirectory);
            }

            string fileName = Path.GetFileNameWithoutExtension(filePath);
            string extension = Path.GetExtension(filePath);
            string backupFileName = $"{fileName}{extension}.bin";
            string backupPath = Path.Combine(backupDirectory, backupFileName);

            File.Copy(filePath, backupPath, true);
            Log.Information("File backed up to: {BackupPath}", backupPath);
            
            return true;
        } catch (Exception ex) {
            Log.Error(ex, "Error backing up file: {FilePath}", filePath);
            return false;
        }
    }

    private bool ReplaceMacroContent(string filePath, string fileExtension) {
        try {
            // NPOI does NOT support writing macro, dead end.
            if (IsLegacyExcelFormat(fileExtension)) {
                // return ReplaceMacroContentLegacy(filePath);
            } else {
                // return ReplaceMacroContentModern(filePath);
            }
        } catch (Exception ex) {
            Log.Error(ex, "Error replacing macro content: {FilePath}", filePath);
            return false;
        }
    }

    private bool ReplaceMacroContentLegacy(string filePath) {
        try {
            using var fileStream = new FileStream(filePath, FileMode.Open, FileAccess.ReadWrite);
            var workbook = new HSSFWorkbook(fileStream);
            
            var vbaProject = workbook.GetVBAProject();
            if (vbaProject != null) {
                // Clear VBA project and replace with sanitized comment
                // Note: NPOI has limited VBA manipulation capabilities
                // This is a simplified approach
                workbook.RemoveVBAProject();
                
                // Add a comment to the first sheet indicating sanitization
                var sheet = workbook.GetSheetAt(0);
                if (sheet != null) {
                    var cell = sheet.GetRow(0)?.GetCell(0) ?? sheet.CreateRow(0).CreateCell(0);
                    cell.SetCellValue("' Sanitized by CRAMC :)");
                }
            }

            fileStream.SetLength(0);
            fileStream.Position = 0;
            workbook.Write(fileStream);
            return true;
        } catch (Exception ex) {
            Log.Error(ex, "Error replacing macro content in legacy Excel file: {FilePath}", filePath);
            return false;
        }
    }

    private bool ReplaceMacroContentModern(string filePath) {
        try {
            using var fileStream = new FileStream(filePath, FileMode.Open, FileAccess.ReadWrite);
            var workbook = new XSSFWorkbook(fileStream);
            
            var vbaProject = workbook.GetVBAProject();
            if (vbaProject != null) {
                // Clear VBA project and replace with sanitized comment
                workbook.RemoveVBAProject();
                
                // Add a comment to the first sheet indicating sanitization
                var sheet = workbook.GetSheetAt(0);
                if (sheet != null) {
                    var cell = sheet.GetRow(0)?.GetCell(0) ?? sheet.CreateRow(0).CreateCell(0);
                    cell.SetCellValue("' Sanitized by CRAMC :)");
                }
            }

            fileStream.SetLength(0);
            fileStream.Position = 0;
            workbook.Write(fileStream);
            return true;
        } catch (Exception ex) {
            Log.Error(ex, "Error replacing macro content in modern Excel file: {FilePath}", filePath);
            return false;
        }
    }

    private string RenameFileWithSuffix(string filePath) {
        try {
            string directory = Path.GetDirectoryName(filePath);
            string fileName = Path.GetFileNameWithoutExtension(filePath);
            string extension = Path.GetExtension(filePath);
            
            string newFileName = $"{fileName}-S{extension}";
            string newPath = Path.Combine(directory, newFileName);
            
            if (File.Exists(newPath)) {
                File.Delete(newPath);
            }
            
            File.Move(filePath, newPath);
            Log.Information("File renamed to: {NewPath}", newPath);
            
            return newPath;
        } catch (Exception ex) {
            Log.Error(ex, "Error renaming file: {FilePath}", filePath);
            return string.Empty;
        }
    }
}