#include <unistd.h>
#include <stdio.h>
#include <string.h>
#include <getopt.h>
#include <unistd.h>
#include <stdlib.h>
#include <sys/ioctl.h>
#include <linux/hdreg.h>
#include <scsi/scsi.h>
#include <scsi/sg.h>
#include <scsi/scsi_ioctl.h>
#include <fcntl.h>

#define DEF(X) 1
int debug = 0;

#if DEF(ATA)
typedef unsigned short u16;
#define swapb(x)                                       \
  ({                                                   \
    u16 __x = (x);                                     \
    (x) = ((u16)((((u16)(__x) & (u16)0x00ffU) << 8) |  \
                 (((u16)(__x) & (u16)0xff00U) >> 8))); \
  })
#define GBUF_SIZE 65535
#define DEFAULT_ATTRIBUTE_ID 194
#define DEFAULT_ATTRIBUTE_ID2 190
#define SBUFF_SIZE 512
static char sbuff[SBUFF_SIZE];

static int ata_probe(int device)
{
  if (device == -1 || ioctl(device, HDIO_GET_IDENTITY, sbuff))
    return 0;
  else
    return 1;
}

int ata_enable_smart(int device)
{
  unsigned char cmd[4] = {WIN_SMART, 0, SMART_ENABLE, 0};

  return ioctl(device, HDIO_DRIVE_CMD, cmd);
}

int ata_get_smart_values(int device, unsigned char *buff)
{
  unsigned char cmd[516] = {WIN_SMART, 0, SMART_READ_VALUES, 1};
  int ret;

  ret = ioctl(device, HDIO_DRIVE_CMD, cmd);
  if (ret)
    return ret;
  memcpy(buff, cmd + 4, 512);
  return 0;
}

static char *ata_model(int device)
{
  if (device == -1 || ioctl(device, HDIO_GET_IDENTITY, sbuff))
    return strdup("unknown");
  else
    return strdup((char *)((u16 *)sbuff + 27));
}

unsigned char *ata_search_temperature(const unsigned char *smart_data, int attribute_id)
{
  int i, n;

  n = 3;
  i = 0;
  if (debug)
    printf("============ ata ============\n");
  while ((debug || *(smart_data + n) != attribute_id) && i < 30)
  {
    if (debug && *(smart_data + n))
      printf("field(%d)\t = %d\t(0x%02x)\n",
             (int)*(smart_data + n),
             (int)*(smart_data + n + 3),
             *(smart_data + n + 3));

    n += 12;
    i++;
  }

  if (i >= 30)
    return NULL;
  else
    return (unsigned char *)(smart_data + n);
}

int ata_get_temperature(int fd)
{
  unsigned char values[512] /*, thresholds[512]*/;
  unsigned char *field;
  int i;
  unsigned short *p;

  if (ata_enable_smart(fd) != 0)
  {
    printf("ATA S.M.A.R.T. not available!\n");
    return -1;
  }

  if (ata_get_smart_values(fd, values))
  {
    printf("ATA Enable S.M.A.R.T. err!\n");
    return -1;
  }

  p = (u16 *)values;
  for (i = 0; i < 256; i++)
  {
    swapb(*(p + i));
  }

  /* get SMART threshold values */
  /*
  if(get_smart_threshold_values(fd, thresholds)) {
    perror("ioctl");
    exit(3);
  }

  p = (u16*)thresholds;
  for(i = 0; i < 256; i++) {
    swapb(*(p+i));
  }
  */

  /* temperature */
  field = ata_search_temperature(values, DEFAULT_ATTRIBUTE_ID);
  if (!field)
    field = ata_search_temperature(values, DEFAULT_ATTRIBUTE_ID2);

  if (field)
    return *(field + 3);
  else
    return -1;
}

#endif

#if DEF(SCSI)
#define TEMPERATURE_PAGE 0x0d
#define CDB_12_HDR_SIZE 14
#define CDB_12_MAX_DATA_SIZE 0xffffffff
#define CDB_6_HDR_SIZE 14
#define CDB_6_MAX_DATA_SIZE 0xff

#define DEXCPT_DISABLE 0xf7
#define DEXCPT_ENABLE 0x08
#define EWASC_ENABLE 0x10
#define EWASC_DISABLE 0xef
#define GBUF_SIZE 65535
#define MODE_DATA_HDR_SIZE 12
#define SMART_SUPPORT 0x00

struct cdb10hdr
{
  unsigned int inbufsize;
  unsigned int outbufsize;
  unsigned int cdb[10];
};

struct cdb6hdr
{
  unsigned int inbufsize;
  unsigned int outbufsize;
  unsigned char cdb[6];
};

static void scsi_fixstring(unsigned char *s, int bytecount)
{
  unsigned char *p;
  unsigned char *end;

  p = s;
  end = s + bytecount;

  /* strip leading blanks */
  while (s != end && *s == ' ')
    ++s;
  /* compress internal blanks and strip trailing blanks */
  while (s != end && *s)
  {
    if (*s++ != ' ' || (s != end && *s && *s != ' '))
      *p++ = *(s - 1);
  }
  /* wipe out trailing garbage */
  while (p != end)
    *p++ = '\0';
}

int scsi_SG_IO(int device, unsigned char *cdb, int cdb_len, unsigned char *buffer, int buffer_len, unsigned char *sense, unsigned char sense_len, int dxfer_direction)
{
  struct sg_io_hdr io_hdr;

  memset(&io_hdr, 0, sizeof(struct sg_io_hdr));
  io_hdr.interface_id = 'S';
  io_hdr.cmdp = cdb;
  io_hdr.cmd_len = cdb_len;
  io_hdr.dxfer_len = buffer_len;
  io_hdr.dxferp = buffer;
  io_hdr.mx_sb_len = sense_len;
  io_hdr.sbp = sense;
  io_hdr.dxfer_direction = dxfer_direction;
  io_hdr.timeout = 3000; /* 3 seconds should be ample */

  return ioctl(device, SG_IO, &io_hdr);
}

int scsi_SEND_COMMAND(int device, unsigned char *cdb, int cdb_len, unsigned char *buffer, int buffer_len, int dxfer_direction)
{
  unsigned char buf[2048];
  unsigned int inbufsize, outbufsize, ret;

  switch (dxfer_direction)
  {
  case SG_DXFER_FROM_DEV:
    inbufsize = 0;
    outbufsize = buffer_len;
    break;
  case SG_DXFER_TO_DEV:
    inbufsize = buffer_len;
    outbufsize = 0;
    break;
  default:
    inbufsize = 0;
    outbufsize = 0;
    break;
  }
  memcpy(buf, &inbufsize, sizeof(inbufsize));
  memcpy(buf + sizeof(inbufsize), &outbufsize, sizeof(outbufsize));
  memcpy(buf + sizeof(inbufsize) + sizeof(outbufsize), cdb, cdb_len);
  memcpy(buf + sizeof(inbufsize) + sizeof(outbufsize) + cdb_len, buffer, buffer_len);

  ret = ioctl(device, SCSI_IOCTL_SEND_COMMAND, buf);

  memcpy(buffer, buf + sizeof(inbufsize) + sizeof(outbufsize), buffer_len);

  return ret;
}

int scsi_command(int device, unsigned char *cdb, int cdb_len, unsigned char *buffer, int buffer_len, int dxfer_direction)
{
  static int SG_IO_supported = -1;
  int ret;

  if (SG_IO_supported == 1)
    return scsi_SG_IO(device, cdb, cdb_len, buffer, buffer_len, NULL, 0, dxfer_direction);
  else if (SG_IO_supported == 0)
    return scsi_SEND_COMMAND(device, cdb, cdb_len, buffer, buffer_len, dxfer_direction);
  else
  {
    ret = scsi_SG_IO(device, cdb, cdb_len, buffer, buffer_len, NULL, 0, dxfer_direction);
    if (ret == 0)
    {
      SG_IO_supported = 1;
      return ret;
    }
    else
    {
      SG_IO_supported = 0;
      return scsi_SEND_COMMAND(device, cdb, cdb_len, buffer, buffer_len, dxfer_direction);
    }
  }
}

int scsi_inquiry(int device, unsigned char *buffer)
{
  unsigned char cdb[6];

  memset(cdb, 0, sizeof(cdb));
  cdb[0] = INQUIRY;
  cdb[4] = 36; /* should be 36 for unsafe devices (like USB mass storage stuff)
                *      otherwise they can lock up! SPC sections 7.4 and 8.6 */

  if (scsi_command(device, cdb, sizeof(cdb), buffer, cdb[4], SG_DXFER_FROM_DEV) != 0)
    return 1;
  else
  {
    scsi_fixstring(buffer + 8, 24);
    return 0;
  }
}

unsigned char modesense(int device, unsigned char pagenum, unsigned char *pBuf)
{

  unsigned char tBuf[CDB_6_MAX_DATA_SIZE + CDB_6_HDR_SIZE];

  struct cdb6hdr *ioctlhdr;

  unsigned char status;

  memset(&tBuf, 0, CDB_6_MAX_DATA_SIZE + CDB_6_HDR_SIZE);

  ioctlhdr = (struct cdb6hdr *)&tBuf;

  ioctlhdr->inbufsize = 0;
  ioctlhdr->outbufsize = 0xff;

  ioctlhdr->cdb[0] = MODE_SENSE;
  ioctlhdr->cdb[1] = 0x00;
  ioctlhdr->cdb[2] = pagenum;
  ioctlhdr->cdb[3] = 0x00;
  ioctlhdr->cdb[4] = CDB_6_MAX_DATA_SIZE;
  ioctlhdr->cdb[5] = 0x00;

  status = ioctl(device, 1, &tBuf);

  memcpy(pBuf, &tBuf[8], 256);

  return status;
}

unsigned char modeselect(int device, unsigned char pagenum, unsigned char *pBuf)
{
  struct cdb6hdr *ioctlhdr;
  unsigned char tBuf[CDB_6_MAX_DATA_SIZE + CDB_6_HDR_SIZE];
  unsigned char status;

  memset(&tBuf, 0, CDB_6_MAX_DATA_SIZE + CDB_6_HDR_SIZE);

  ioctlhdr = (struct cdb6hdr *)&tBuf;

  ioctlhdr->inbufsize = pBuf[0] + 1;
  ioctlhdr->outbufsize = 0;

  ioctlhdr->cdb[0] = MODE_SELECT;
  ioctlhdr->cdb[1] = 0x11;
  ioctlhdr->cdb[2] = 0x00;
  ioctlhdr->cdb[3] = 0x00;
  ioctlhdr->cdb[4] = pBuf[0] + 1;
  ioctlhdr->cdb[5] = 0x00;

  tBuf[CDB_6_HDR_SIZE + 3] = 0x08;
  tBuf[CDB_6_HDR_SIZE + 10] = 0x02;

  memcpy(&tBuf[CDB_6_HDR_SIZE + MODE_DATA_HDR_SIZE],
         pBuf + MODE_DATA_HDR_SIZE,
         pBuf[0] - MODE_DATA_HDR_SIZE + 1);

  tBuf[26] &= 0x3f;

  status = ioctl(device, 1, &tBuf);

  return status;
}

unsigned char scsi_smart_mode_page1c_handler(int device, unsigned char setting, unsigned char *retval)
{
  char tBuf[CDB_6_MAX_DATA_SIZE];

  if (modesense(device, 0x1c, (unsigned char *)&tBuf) != 0)
  {
    return 1;
  }

  switch (setting)
  {
  case DEXCPT_DISABLE:
    tBuf[14] &= 0xf7;
    tBuf[15] = 0x04;
    break;
  case DEXCPT_ENABLE:
    tBuf[14] |= 0x08;
    break;
  case EWASC_ENABLE:
    tBuf[14] |= 0x10;
    break;
  case EWASC_DISABLE:
    tBuf[14] &= 0xef;
    break;
  case SMART_SUPPORT:
    *retval = tBuf[14] & 0x08;
    return 0;
    break;
  default:
    return 1;
  }

  if (modeselect(device, 0x1c, (unsigned char *)&tBuf) != 0)
  {
    return 1;
  }

  return 0;
}

unsigned char log_sense(int device, unsigned char pagenum, unsigned char *pBuf)
{
  struct cdb10hdr *ioctlhdr;
  unsigned char tBuf[1024 + CDB_12_HDR_SIZE];
  unsigned char status;

  memset(&tBuf, 0, 255);

  ioctlhdr = (struct cdb10hdr *)tBuf;

  ioctlhdr->inbufsize = 0;
  ioctlhdr->outbufsize = 1024;

  ioctlhdr->cdb[0] = LOG_SENSE;
  ioctlhdr->cdb[1] = 0x00;
  ioctlhdr->cdb[2] = 0x40 | pagenum;
  ioctlhdr->cdb[3] = 0x00;
  ioctlhdr->cdb[4] = 0x00;
  ioctlhdr->cdb[5] = 0x00;
  ioctlhdr->cdb[6] = 0x00;
  ioctlhdr->cdb[7] = 0x04;
  ioctlhdr->cdb[8] = 0x00;
  ioctlhdr->cdb[9] = 0x00;

  status = ioctl(device, 1, &tBuf);

  memcpy(pBuf, &tBuf[8], 1024);

  return status;
}

static int scsi_probe(int device)
{
  int bus_num;

  if (ioctl(device, SCSI_IOCTL_GET_BUS_NUMBER, &bus_num))
    return 0;
  else
    return 1;
}

static char *scsi_model(int device)
{
  unsigned char buf[36];

  if (scsi_inquiry(device, buf) != 0)
    return strdup("unknown");
  else
  {
    return strdup(buf + 8);
  }
}

int scsi_get_temperature(int fd)
{
  unsigned char buf[1024];
  unsigned char smartsupport;
  char gBuf[GBUF_SIZE];

  if (0 != scsi_smart_mode_page1c_handler(fd, SMART_SUPPORT, &smartsupport))
  {
    printf("SCSI S.M.A.R.T. not available!\n");
    return -1;
  }

  if (0 != scsi_smart_mode_page1c_handler(fd, DEXCPT_DISABLE, NULL))
  {
    printf("SCSI Enable S.M.A.R.T. err!\n");
    return -1;
  }

  if (log_sense(fd, TEMPERATURE_PAGE, buf) != 0)
  {
    printf("SCSI read err!\n");
    return -1;
  }
  return buf[9];
}

#endif

#if DEF(SATA)
#ifndef ATA_16
/* Values for T10/04-262r7 */
#define ATA_16 0x85 /* 16-byte pass-thru */
#endif

int sata_pass_thru(int device, unsigned char *cmd, unsigned char *buffer)
{
  unsigned char cdb[16];
  unsigned char sense[32];
  int dxfer_direction;
  int ret;

  memset(cdb, 0, sizeof(cdb));
  cdb[0] = ATA_16;
  if (cmd[3])
  {
    cdb[1] = (4 << 1); /* PIO Data-in */
    cdb[2] = 0x2e;     /* no off.line, cc, read from dev, lock count in sector count field */
    dxfer_direction = SG_DXFER_FROM_DEV;
  }
  else
  {
    cdb[1] = (3 << 1); /* Non-data */
    cdb[2] = 0x20;     /* cc */
    dxfer_direction = SG_DXFER_NONE;
  }
  cdb[4] = cmd[2];
  if (cmd[0] == WIN_SMART)
  {
    cdb[6] = cmd[3];
    cdb[8] = cmd[1];
    cdb[10] = 0x4f;
    cdb[12] = 0xc2;
  }
  else
    cdb[6] = cmd[1];
  cdb[14] = cmd[0];

  ret = scsi_SG_IO(device, cdb, sizeof(cdb), buffer, cmd[3] * 512, sense, sizeof(sense), dxfer_direction);

  /* Verify SATA magics */
  if (sense[0] != 0x72)
    return 1;
  else
    return ret;
}

void sata_fixstring(unsigned char *s, int bytecount)
{
  unsigned char *p;
  unsigned char *end;

  p = s;
  end = &s[bytecount & ~1]; /* bytecount must be even */

  /* convert from big-endian to host byte order */
  for (p = end; p != s;)
  {
    unsigned short *pp = (unsigned short *)(p -= 2);
    *pp = ntohs(*pp);
  }

  /* strip leading blanks */
  while (s != end && *s == ' ')
    ++s;
  /* compress internal blanks and strip trailing blanks */
  while (s != end && *s)
  {
    if (*s++ != ' ' || (s != end && *s && *s != ' '))
      *p++ = *(s - 1);
  }
  /* wipe out trailing garbage */
  while (p != end)
    *p++ = '\0';
}

static int sata_probe(int device)
{
  int bus_num;
  unsigned char cmd[4] = {WIN_IDENTIFY, 0, 0, 1};
  unsigned char identify[512];
  char buf[36]; /* should be 36 for unsafe devices (like USB mass storage stuff)
                        otherwise they can lock up! SPC sections 7.4 and 8.6 */

  /* SATA disks are difficult to detect as they answer to both ATA and SCSI
     commands */

  /* First check that the device is accessible through SCSI */
  if (ioctl(device, SCSI_IOCTL_GET_BUS_NUMBER, &bus_num))
    return 0;

  /* Get SCSI name and verify it starts with "ATA " */
  if (scsi_inquiry(device, buf))
    return 0;
  else if (strncmp(buf + 8, "ATA ", 4))
    return 0;

  /* Verify that it supports ATA pass thru */
  if (sata_pass_thru(device, cmd, identify) != 0)
    return 0;
  else
    return 1;
}

int sata_enable_smart(int device)
{
  unsigned char cmd[4] = {WIN_SMART, 0, SMART_ENABLE, 0};

  return sata_pass_thru(device, cmd, NULL);
}

int sata_get_smart_values(int device, unsigned char *buff)
{
  unsigned char cmd[4] = {WIN_SMART, 0, SMART_READ_VALUES, 1};

  return sata_pass_thru(device, cmd, buff);
}

static char *sata_model(int device)
{
  unsigned char cmd[4] = {WIN_IDENTIFY, 0, 0, 1};
  unsigned char identify[512];

  if (device == -1 || sata_pass_thru(device, cmd, identify))
    return strdup("unknown");
  else
  {
    sata_fixstring(identify + 54, 40);
    return strdup(identify + 54);
  }
}

static unsigned char *sata_search_temperature(const unsigned char *smart_data, int attribute_id)
{
  int i, n;

  n = 3;
  i = 0;
  if (debug)
    printf("============ sata ============\n");
  while ((debug || *(smart_data + n) != attribute_id) && i < 30)
  {
    if (debug && *(smart_data + n))
      printf("field(%d)\t = %d\t(0x%02x)\n", *(smart_data + n), *(smart_data + n + 3), *(smart_data + n + 3));

    n += 12;
    i++;
  }

  if (i >= 30)
    return NULL;
  else
    return (unsigned char *)(smart_data + n);
}

int sata_get_temperature(int fd)
{
  unsigned char values[512];
  unsigned char *field;
  int i;
  u16 *p;
  /* get SMART values */
  if (sata_enable_smart(fd) != 0)
  {
    printf("SATA S.M.A.R.T. not available!\n");
    return -1;
  }

  if (sata_get_smart_values(fd, values))
  {
    printf("SATA Enable S.M.A.R.T. err!\n");
    return -1;
  }

  p = (u16 *)values;
  for (i = 0; i < 256; i++)
  {
    swapb(*(p + i));
  }

  /* temperature */
  field = sata_search_temperature(values, DEFAULT_ATTRIBUTE_ID);
  if (!field)
    field = sata_search_temperature(values, DEFAULT_ATTRIBUTE_ID2);

  if (field)
    return *(field + 3);
  else
    return -1;
}

#endif

int print_usage()
{
  printf("Usage:\n");
  printf("          satatemp  [-d]  /dev/sda\n");
  printf("          -d print debug  info\n");
  return 0;
}

int hddtemp(char *device)
{
  int fd = 0;
  int value = -1;
  char type[16] = "";
  char *mode = NULL;

  fd = open(device, O_RDONLY | O_NONBLOCK);
  if (fd < 0)
  {
    printf("open err!\n");
    return (-1);
  }

  if (sata_probe(fd))
  {
    value = sata_get_temperature(fd);
    memset(type, 0, sizeof(type));
    strcpy(type, "SATA mode");
    mode = sata_model(fd);
  }
  else if (ata_probe(fd))
  {
    value = ata_get_temperature(fd);
    memset(type, 0, sizeof(type));
    strcpy(type, "ATA mode");
    mode = ata_model(fd);
  }
  else if (scsi_probe(fd))
  {
    value = scsi_get_temperature(fd);
    memset(type, 0, sizeof(type));
    strcpy(type, "SCSI mode");
    mode = scsi_model(fd);
  }

  close(fd);
  free(mode);
  return value;
}
